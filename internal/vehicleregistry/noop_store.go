package vehicleregistry

// NoopVehicleStore is a no-op implementation used when no database is available.
type NoopVehicleStore struct{}

func (NoopVehicleStore) List() ([]Vehicle, error)                          { return nil, nil }
func (NoopVehicleStore) Add(_, _, _ string) error                          { return nil }
func (NoopVehicleStore) Delete(_ string) error                             { return nil }
func (NoopVehicleStore) Exists(_ string) (bool, error)                     { return false, nil }
func (NoopVehicleStore) SeedDefault() error                                { return nil }
