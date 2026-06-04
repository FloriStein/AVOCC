// Package telemetryservice implements the MQTT Telemetry Bridge (BE-05, ADR-003/008/016).
// It subscribes to vehicle telemetry topics on Mosquitto and holds the latest
// TelemetryEvent per vehicle for the control server to query.
package telemetryservice

import (
	"log"
	"sync"
	"time"

	telemetryv1 "avoc/gen/go/telemetry/v1"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"google.golang.org/protobuf/proto"
)

const (
	topicVehicleTelemetry = "vehicle/+/telemetry"
	reconnectDelay        = 5 * time.Second
)

// Client manages the MQTT connection and caches the latest TelemetryEvent per vehicle.
type Client struct {
	broker string
	client mqtt.Client
	mu     sync.RWMutex
	latest map[string]*telemetryv1.TelemetryEvent
}

func NewClient(broker string) *Client {
	return &Client{
		broker: broker,
		latest: make(map[string]*telemetryv1.TelemetryEvent),
	}
}

// Connect establishes the MQTT connection with automatic reconnect.
func (c *Client) Connect() error {
	opts := mqtt.NewClientOptions().
		AddBroker("tcp://" + c.broker).
		SetClientID("avoc-telemetry-service").
		SetAutoReconnect(true).
		SetConnectRetryInterval(reconnectDelay).
		SetOnConnectHandler(func(_ mqtt.Client) {
			log.Printf("[MQTT] connected to %s", c.broker)
			c.subscribe()
		}).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			log.Printf("[MQTT] connection lost: %v", err)
		})

	c.client = mqtt.NewClient(opts)
	token := c.client.Connect()
	token.Wait()
	return token.Error()
}

func (c *Client) subscribe() {
	token := c.client.Subscribe(topicVehicleTelemetry, 1, c.handleMessage)
	token.Wait()
	if err := token.Error(); err != nil {
		log.Printf("[MQTT] subscribe error: %v", err)
		return
	}
	log.Printf("[MQTT] subscribed to %s", topicVehicleTelemetry)
}

func (c *Client) handleMessage(_ mqtt.Client, msg mqtt.Message) {
	event := &telemetryv1.TelemetryEvent{}
	if err := proto.Unmarshal(msg.Payload(), event); err != nil {
		log.Printf("[MQTT] parse error on topic %s: %v", msg.Topic(), err)
		return
	}

	vehicleID := ""
	if event.Header != nil {
		vehicleID = event.Header.VehicleId
	}
	if vehicleID == "" {
		return
	}

	c.mu.Lock()
	c.latest[vehicleID] = event
	c.mu.Unlock()

	log.Printf("[MQTT] telemetry: vehicle=%s speed=%.1f km/h battery=%.0f%%",
		vehicleID, event.SpeedKmh, event.BatteryPct)
}

// GetLatest returns the most recent TelemetryEvent for the given vehicle.
func (c *Client) GetLatest(vehicleID string) (*telemetryv1.TelemetryEvent, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.latest[vehicleID]
	return e, ok
}

// Disconnect closes the MQTT connection cleanly.
func (c *Client) Disconnect() {
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(250)
	}
}
