package main

import (
	"context"
	"net/http"
	"os"

	"avoc/internal/authservice"
	pkgdb "avoc/pkg/db"
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

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		log.Fatal("ADMIN_PASSWORD environment variable is required")
	}

	db, err := pkgdb.Open(databaseURL)
	if err != nil {
		log.Fatal("failed to open database", "error", err)
	}
	defer db.Close()

	userStore, err := authservice.NewPostgresUserStore(db)
	if err != nil {
		log.Fatal("failed to initialize user store", "error", err)
	}

	ctx := context.Background()
	if err := userStore.SeedAdmin(ctx, "admin", adminPassword); err != nil {
		log.Fatal("failed to seed admin user", "error", err)
	}
	log.Info("admin user seeded (idempotent)")

	handler := authservice.NewHandler(secret, userStore)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/operator/login", handler.OperatorLogin)
	mux.HandleFunc("POST /auth/vehicle/register", handler.VehicleRegister)
	mux.HandleFunc("POST /auth/token/validate", handler.ValidateToken)
	mux.HandleFunc("POST /auth/token/refresh", handler.RefreshToken)
	mux.HandleFunc("POST /auth/handover/token", handler.HandoverToken)

	// User management — ADMIN only (ADR-024)
	mux.HandleFunc("GET /auth/users",         handler.RequireAdmin(handler.ListUsers))
	mux.HandleFunc("POST /auth/users",        handler.RequireAdmin(handler.CreateUser))
	mux.HandleFunc("DELETE /auth/users/{id}", handler.RequireAdmin(handler.DeleteUser))
	mux.HandleFunc("PATCH /auth/users/{id}",  handler.RequireAdmin(handler.UpdateUserRole))

	mux.HandleFunc("GET /health", handler.Health)

	log.Info("Auth Service starting", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal("Auth Service failed", "error", err)
	}
}
