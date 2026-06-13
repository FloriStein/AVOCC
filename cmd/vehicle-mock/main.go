// vehicle-mock simulates a vehicle client for development and testing (ADR-021).
//
// It implements the same interface a real Raspberry Pi would use:
//   - Connects to Control Server on /vehicle/ws with a VEHICLE JWT
//   - Receives ControlCommand (Protobuf) and sends back VehicleCommandAck
//   - Publishes TelemetryEvent (MQTT) with commanded + simulated actual actuation values
//
// Usage (via Docker Compose):
//
//	VEHICLE_ID=vehicle-001 CONTROL_SERVER_URL=ws://control-server:8080/vehicle/ws
//	JWT_SECRET=... MQTT_BROKER=mosquitto:1883
package main

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	commonv1 "avoc/gen/go/common/v1"
	controlv1 "avoc/gen/go/control/v1"
	telemetryv1 "avoc/gen/go/telemetry/v1"
	vehiclev1 "avoc/gen/go/vehicle/v1"
	"avoc/pkg/logger"
	"avoc/pkg/ulid"

	"github.com/golang-jwt/jwt/v5"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

var log = logger.New("vehicle-mock")

const (
	reconnectDelay  = 5 * time.Second
	telemetryHz     = 2 * time.Second // publish telemetry every 2s
	actuatorLag     = 0.15            // simulated lag: actual trails commanded by 15%
)

type state struct {
	steerCommanded    float64
	throttleCommanded float64
	brakeCommanded    float64
	steerActual       float64
	throttleActual    float64
	speedKmh          float64
	battery           float64
	sessionID         string
}

func main() {
	vehicleID := envOr("VEHICLE_ID", "vehicle-001")
	wsURL := envOr("CONTROL_SERVER_URL", "ws://control-server:8080/vehicle/ws")
	jwtSecret := os.Getenv("JWT_SECRET")
	mqttBroker := envOr("MQTT_BROKER", "mosquitto:1883")

	if jwtSecret == "" {
		log.Fatal("JWT_SECRET required")
	}

	token, err := makeVehicleJWT(vehicleID, jwtSecret)
	if err != nil {
		log.Fatal("JWT generation failed", "error", err)
	}

	mqttClient := connectMQTT(mqttBroker, vehicleID)
	defer mqttClient.Disconnect(250)

	st := &state{battery: 85.0}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			select {
			case <-sig:
				return
			default:
			}
			if err := runConnection(wsURL, token, vehicleID, mqttClient, st); err != nil {
				log.Warn("connection lost — reconnecting", "error", err, "delay", reconnectDelay)
				time.Sleep(reconnectDelay)
			}
		}
	}()

	<-sig
	log.Info("vehicle-mock shutting down")
}

func runConnection(wsURL, token, vehicleID string, mqttClient mqtt.Client, st *state) error {
	header := map[string][]string{
		"Authorization": {"Bearer " + token},
	}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	log.Info("vehicle connected to control server", "vehicle_id", vehicleID, "url", wsURL)

	// Telemetry publish loop
	ticker := time.NewTicker(telemetryHz)
	defer ticker.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ticker.C {
			publishTelemetry(mqttClient, vehicleID, st)
		}
	}()

	// Command receive loop
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		cmd := &controlv1.ControlCommand{}
		if err := proto.Unmarshal(data, cmd); err != nil {
			log.Warn("cannot decode command", "error", err)
			continue
		}

		applyCommand(cmd, st)

		if cmd.Header != nil {
			if sid := cmd.Header.SessionId; sid != "" {
				st.sessionID = sid
			}
		}

		// Send VehicleCommandAck
		eventID := ""
		if cmd.Header != nil {
			eventID = cmd.Header.EventId
		}
		ack := &vehiclev1.VehicleCommandAck{
			Header: &commonv1.CorrelationHeader{
				EventId:   ulid.Generate(),
				VehicleId: vehicleID,
				Timestamp: time.Now().UnixMilli(),
				SessionId: st.sessionID,
			},
			CommandEventId: eventID,
			Received:       true,
			ReceivedAtMs:   time.Now().UnixMilli(),
		}
		ackBytes, err := proto.Marshal(ack)
		if err != nil {
			log.Warn("ack marshal failed", "error", err)
			continue
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, ackBytes); err != nil {
			return fmt.Errorf("ack write: %w", err)
		}
		log.Debug("command received + ACK sent",
			"type", cmd.Type, "value", cmd.Value, "event_id", eventID)
	}
}

func applyCommand(cmd *controlv1.ControlCommand, st *state) {
	switch cmd.Type {
	case controlv1.CommandType_COMMAND_TYPE_STEER:
		st.steerCommanded = float64(cmd.Value)
		st.steerActual = lerp(st.steerActual, st.steerCommanded, 1.0-actuatorLag)
	case controlv1.CommandType_COMMAND_TYPE_THROTTLE:
		st.throttleCommanded = float64(cmd.Value)
		st.throttleActual = lerp(st.throttleActual, st.throttleCommanded, 1.0-actuatorLag)
		st.speedKmh = math.Abs(st.throttleActual) * 30.0 // max 30 km/h for mock
	case controlv1.CommandType_COMMAND_TYPE_BRAKE:
		st.brakeCommanded = float64(cmd.Value)
		st.speedKmh = math.Max(0, st.speedKmh-st.brakeCommanded*5.0)
	}
}

func publishTelemetry(mqttClient mqtt.Client, vehicleID string, st *state) {
	event := &telemetryv1.TelemetryEvent{
		Header: &commonv1.CorrelationHeader{
			EventId:   ulid.Generate(),
			VehicleId: vehicleID,
			Timestamp: time.Now().UnixMilli(),
			SessionId: st.sessionID,
		},
		SpeedKmh:          st.speedKmh,
		BatteryPct:        st.battery,
		Status:            "MOCK_RUNNING",
		SteerCommanded:    st.steerCommanded,
		ThrottleCommanded: st.throttleCommanded,
		BrakeCommanded:    st.brakeCommanded,
		SteerActual:       st.steerActual,
		ThrottleActual:    st.throttleActual,
	}

	data, err := proto.Marshal(event)
	if err != nil {
		log.Warn("telemetry marshal failed", "error", err)
		return
	}

	topic := fmt.Sprintf("vehicle/%s/telemetry", vehicleID)
	token := mqttClient.Publish(topic, 1, false, data)
	token.Wait()
	if err := token.Error(); err != nil {
		log.Warn("MQTT publish failed", "error", err)
	}
}

func connectMQTT(broker, vehicleID string) mqtt.Client {
	opts := mqtt.NewClientOptions().
		AddBroker("tcp://" + broker).
		SetClientID("avoc-vehicle-mock-" + vehicleID).
		SetAutoReconnect(true).
		SetConnectRetryInterval(reconnectDelay).
		SetOnConnectHandler(func(_ mqtt.Client) {
			log.Info("MQTT connected", "broker", broker)
		}).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			log.Warn("MQTT connection lost", "error", err)
		})
	client := mqtt.NewClient(opts)
	for {
		token := client.Connect()
		token.Wait()
		if token.Error() == nil {
			return client
		}
		log.Warn("MQTT connect failed — retrying", "error", token.Error())
		time.Sleep(reconnectDelay)
	}
}

func makeVehicleJWT(vehicleID, secret string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  vehicleID,
		"role": "VEHICLE",
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func lerp(current, target, factor float64) float64 {
	return current + (target-current)*factor
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
