package authservice

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

const usersSchema = `
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    display_name  TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'OBSERVER',
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_auth_at  TIMESTAMPTZ
);`

// User represents a system operator account.
type User struct {
	ID          string
	DisplayName string
	Role        OperatorRole
	IsActive    bool
	CreatedAt   time.Time
	LastAuthAt  *time.Time
}

// UserStore defines persistence operations for operator accounts (ADR-024).
type UserStore interface {
	Create(ctx context.Context, id, displayName, password string, role OperatorRole) error
	Authenticate(ctx context.Context, id, password string) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)
	List(ctx context.Context) ([]User, error)
	Delete(ctx context.Context, id string) error
	UpdateRole(ctx context.Context, id string, role OperatorRole) error
	SeedAdmin(ctx context.Context, id, password string) error
}

// PostgresUserStore implements UserStore against PostgreSQL (ADR-023/024).
type PostgresUserStore struct {
	db *sql.DB
}

func NewPostgresUserStore(db *sql.DB) (*PostgresUserStore, error) {
	if _, err := db.Exec(usersSchema); err != nil {
		return nil, fmt.Errorf("userstore: create schema: %w", err)
	}
	return &PostgresUserStore{db: db}, nil
}

func (s *PostgresUserStore) Create(ctx context.Context, id, displayName, password string, role OperatorRole) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("userstore: hash password: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO users (id, display_name, password_hash, role) VALUES ($1, $2, $3, $4)`,
		id, displayName, string(hash), string(role),
	)
	if err != nil {
		return fmt.Errorf("userstore: create: %w", err)
	}
	return nil
}

func (s *PostgresUserStore) Authenticate(ctx context.Context, id, password string) (*User, error) {
	var u User
	var hash string
	var lastAuth sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT id, display_name, password_hash, role, is_active, created_at, last_auth_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.DisplayName, &hash, &u.Role, &u.IsActive, &u.CreatedAt, &lastAuth)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("userstore: invalid credentials")
	}
	if err != nil {
		return nil, fmt.Errorf("userstore: authenticate: %w", err)
	}
	if !u.IsActive {
		return nil, fmt.Errorf("userstore: account deactivated")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, fmt.Errorf("userstore: invalid credentials")
	}
	if lastAuth.Valid {
		u.LastAuthAt = &lastAuth.Time
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE users SET last_auth_at = NOW() WHERE id = $1`, id)
	return &u, nil
}

func (s *PostgresUserStore) FindByID(ctx context.Context, id string) (*User, error) {
	var u User
	var lastAuth sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT id, display_name, role, is_active, created_at, last_auth_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.DisplayName, &u.Role, &u.IsActive, &u.CreatedAt, &lastAuth)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("userstore: find: %w", err)
	}
	if lastAuth.Valid {
		u.LastAuthAt = &lastAuth.Time
	}
	return &u, nil
}

func (s *PostgresUserStore) List(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, display_name, role, is_active, created_at, last_auth_at
		 FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("userstore: list: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var lastAuth sql.NullTime
		if err := rows.Scan(&u.ID, &u.DisplayName, &u.Role, &u.IsActive, &u.CreatedAt, &lastAuth); err != nil {
			return nil, err
		}
		if lastAuth.Valid {
			u.LastAuthAt = &lastAuth.Time
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *PostgresUserStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("userstore: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("userstore: user not found")
	}
	return nil
}

func (s *PostgresUserStore) UpdateRole(ctx context.Context, id string, role OperatorRole) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET role = $1 WHERE id = $2`, string(role), id)
	if err != nil {
		return fmt.Errorf("userstore: update role: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("userstore: user not found")
	}
	return nil
}

// SeedAdmin creates the initial admin account idempotently — safe to call on every startup (ADR-024).
func (s *PostgresUserStore) SeedAdmin(ctx context.Context, id, password string) error {
	if password == "" {
		return fmt.Errorf("userstore: ADMIN_PASSWORD must not be empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("userstore: hash admin password: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO users (id, display_name, password_hash, role)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO NOTHING`,
		id, "Administrator", string(hash), string(RoleAdmin),
	)
	return err
}
