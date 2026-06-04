package main

import (
	"net/http"
	"os"

	"avoc/internal/authservice"
	"avoc/pkg/logger"
)

var log = logger.New("auth-service")

func main() {
	port := os.Getenv("AUTH_PORT")
	if port == "" {
		port = "8081"
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	handler := authservice.NewHandler(secret)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/operator/login", handler.OperatorLogin)
	mux.HandleFunc("POST /auth/vehicle/register", handler.VehicleRegister)
	mux.HandleFunc("POST /auth/token/validate", handler.ValidateToken)
	mux.HandleFunc("POST /auth/token/refresh", handler.RefreshToken)
	mux.HandleFunc("POST /auth/handover/token", handler.HandoverToken)
	mux.HandleFunc("GET /health", handler.Health)

	log.Info("Auth Service starting", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal("Auth Service failed", "error", err)
	}
}
