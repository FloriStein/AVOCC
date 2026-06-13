package vehicleregistry

import (
	"database/sql"
	"fmt"
	"time"
)

const schema = `
CREATE TABLE IF NOT EXISTS vehicles (
    id           TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL
);`

// SQLiteVehicleStore persists the fleet configuration in the shared audit SQLite database.
type SQLiteVehicleStore struct {
	db   *sql.DB
	conn ConnectionChecker
}

func NewSQLiteVehicleStore(db *sql.DB, conn ConnectionChecker) (*SQLiteVehicleStore, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("vehicleregistry: create schema: %w", err)
	}
	return &SQLiteVehicleStore{db: db, conn: conn}, nil
}

func (s *SQLiteVehicleStore) List() ([]Vehicle, error) {
	rows, err := s.db.Query(`SELECT id, display_name, description, created_at FROM vehicles ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("vehicleregistry: list: %w", err)
	}
	defer rows.Close()

	var vehicles []Vehicle
	for rows.Next() {
		var v Vehicle
		var createdAt string
		if err := rows.Scan(&v.ID, &v.DisplayName, &v.Description, &createdAt); err != nil {
			return nil, err
		}
		v.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		v.Online = s.conn.Connected(v.ID)
		vehicles = append(vehicles, v)
	}
	return vehicles, rows.Err()
}

func (s *SQLiteVehicleStore) Add(id, displayName, description string) error {
	_, err := s.db.Exec(
		`INSERT INTO vehicles (id, display_name, description, created_at) VALUES (?, ?, ?, ?)`,
		id, displayName, description, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("vehicleregistry: add: %w", err)
	}
	return nil
}

func (s *SQLiteVehicleStore) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM vehicles WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("vehicleregistry: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteVehicleStore) Exists(id string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM vehicles WHERE id = ?`, id).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("vehicleregistry: exists: %w", err)
	}
	return count > 0, nil
}

// SeedDefault inserts vehicle-001 if the table is empty, keeping backward compatibility
// with the vehicle-mock container (VEHICLE_ID: "vehicle-001").
func (s *SQLiteVehicleStore) SeedDefault() error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO vehicles (id, display_name, description, created_at) VALUES (?, ?, ?, ?)`,
		"vehicle-001", "Vehicle 001", "", time.Now().UTC().Format(time.RFC3339),
	)
	return err
}
