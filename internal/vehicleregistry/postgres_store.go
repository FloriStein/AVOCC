package vehicleregistry

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

const schema = `
CREATE TABLE IF NOT EXISTS vehicles (
    id           TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

// PostgresVehicleStore persists the fleet configuration in PostgreSQL (ADR-023).
type PostgresVehicleStore struct {
	db   *sql.DB
	conn ConnectionChecker
}

func NewPostgresVehicleStore(db *sql.DB, conn ConnectionChecker) (*PostgresVehicleStore, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("vehicleregistry: create schema: %w", err)
	}
	return &PostgresVehicleStore{db: db, conn: conn}, nil
}

func (s *PostgresVehicleStore) List() ([]Vehicle, error) {
	rows, err := s.db.Query(`SELECT id, display_name, description, created_at FROM vehicles ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("vehicleregistry: list: %w", err)
	}
	defer rows.Close()

	var vehicles []Vehicle
	for rows.Next() {
		var v Vehicle
		var createdAt time.Time
		if err := rows.Scan(&v.ID, &v.DisplayName, &v.Description, &createdAt); err != nil {
			return nil, err
		}
		v.CreatedAt = createdAt
		v.Online = s.conn.Connected(v.ID)
		vehicles = append(vehicles, v)
	}
	return vehicles, rows.Err()
}

func (s *PostgresVehicleStore) Add(id, displayName, description string) error {
	_, err := s.db.Exec(
		`INSERT INTO vehicles (id, display_name, description) VALUES ($1, $2, $3)`,
		id, displayName, description,
	)
	if err != nil {
		return fmt.Errorf("vehicleregistry: add: %w", err)
	}
	return nil
}

func (s *PostgresVehicleStore) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM vehicles WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("vehicleregistry: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresVehicleStore) Exists(id string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM vehicles WHERE id = $1`, id).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("vehicleregistry: exists: %w", err)
	}
	return count > 0, nil
}

// SeedDefault inserts vehicle-001 if not present — keeps compatibility with vehicle-mock (ADR-021).
func (s *PostgresVehicleStore) SeedDefault() error {
	_, err := s.db.Exec(
		`INSERT INTO vehicles (id, display_name, description) VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO NOTHING`,
		"vehicle-001", "Vehicle 001", "",
	)
	return err
}
