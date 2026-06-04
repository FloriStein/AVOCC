package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"avoc/internal/telemetryservice"
	"avoc/pkg/logger"
)

var log = logger.New("telemetry-service")

func main() {
	port := os.Getenv("TELEMETRY_PORT")
	if port == "" {
		port = "8083"
	}
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		broker = "mosquitto:1883"
	}

	client := telemetryservice.NewClient(broker)
	if err := client.Connect(); err != nil {
		log.Fatal("MQTT connect failed", "broker", broker, "error", err)
	}
	defer client.Disconnect()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /telemetry/latest/", func(w http.ResponseWriter, r *http.Request) {
		vehicleID := strings.TrimPrefix(r.URL.Path, "/telemetry/latest/")
		if vehicleID == "" {
			http.Error(w, "vehicle_id required", http.StatusBadRequest)
			return
		}
		event, ok := client.GetLatest(vehicleID)
		if !ok {
			http.Error(w, "no telemetry for vehicle", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"vehicle_id":  event.Header.GetVehicleId(),
			"session_id":  event.Header.GetSessionId(),
			"speed_kmh":   event.SpeedKmh,
			"battery_pct": event.BatteryPct,
			"latitude":    event.Latitude,
			"longitude":   event.Longitude,
			"status":      event.Status,
			"timestamp":   event.Header.GetTimestamp(),
		})
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "telemetry-service"})
	})

	log.Info("Telemetry Service starting", "port", port, "broker", broker)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal("Telemetry Service failed", "error", err)
	}
}
