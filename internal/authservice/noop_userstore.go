package authservice

import (
	"context"
	"fmt"
)

// NoopUserStore is a no-op implementation used in tests that don't exercise auth logic.
type NoopUserStore struct{}

func (NoopUserStore) Create(_ context.Context, _, _, _ string, _ OperatorRole) error {
	return fmt.Errorf("noop: not implemented")
}
func (NoopUserStore) Authenticate(_ context.Context, _, _ string) (*User, error) {
	return nil, fmt.Errorf("noop: not implemented")
}
func (NoopUserStore) FindByID(_ context.Context, _ string) (*User, error)     { return nil, nil }
func (NoopUserStore) List(_ context.Context) ([]User, error)                  { return nil, nil }
func (NoopUserStore) Delete(_ context.Context, _ string) error                { return nil }
func (NoopUserStore) UpdateRole(_ context.Context, _ string, _ OperatorRole) error { return nil }
func (NoopUserStore) SeedAdmin(_ context.Context, _, _ string) error          { return nil }
