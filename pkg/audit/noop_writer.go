package audit

// NoopWriter implements AuditWriter as a no-op.
// Use in tests and when no audit storage is configured.
type NoopWriter struct{}

func NewNoopWriter() *NoopWriter { return &NoopWriter{} }

func (n *NoopWriter) WriteSync(_ SafetyAuditEvent) error               { return nil }
func (n *NoopWriter) QueryBySession(_ string) ([]SafetyAuditEvent, error) { return nil, nil }
func (n *NoopWriter) Close() error                                     { return nil }
