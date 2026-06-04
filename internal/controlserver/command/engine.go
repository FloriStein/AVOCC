// Package command implements the Control Input System — Command Routing & Processing (BE-04, ADR-007/010/012b).
// It parses incoming Protobuf ControlCommand messages, routes them by type,
// enforces rate limiting, and returns a Protobuf ControlAck response.
package command

import (
	"sync"
	"time"

	commonv1 "avoc/gen/go/common/v1"
	controlv1 "avoc/gen/go/control/v1"
	"avoc/internal/controlserver/safety"
	"avoc/internal/controlserver/session"
	"avoc/internal/controlserver/statemachine"
	"avoc/internal/safetyservice"
	"avoc/pkg/audit"
	"avoc/pkg/logger"
	"avoc/pkg/ulid"

	"google.golang.org/protobuf/proto"
)

var svcLog = logger.New("control-server")

const maxCommandsPerSecond = 100

// Engine routes parsed ControlCommands to the correct handler (ADR-007).
type Engine struct {
	sm          *statemachine.Machine
	safetyPub   safety.Publisher
	sessionMgr  *session.Manager
	deadman     *safety.DeadmanWatchdog
	auditWriter audit.AuditWriter
	limiter     *tokenBucket
}

func NewEngine(
	sm *statemachine.Machine,
	safetyPub safety.Publisher,
	sessionMgr *session.Manager,
	deadman *safety.DeadmanWatchdog,
) *Engine {
	return &Engine{
		sm:         sm,
		safetyPub:  safetyPub,
		sessionMgr: sessionMgr,
		deadman:    deadman,
		limiter:    newTokenBucket(maxCommandsPerSecond),
	}
}

// WithAuditWriter sets the audit writer for EMERGENCY_STOP persistence (ADR-018).
func (e *Engine) WithAuditWriter(aw audit.AuditWriter) *Engine {
	e.auditWriter = aw
	return e
}

// Handle parses rawMsg as a Protobuf ControlCommand, routes it, and returns
// a serialised ControlAck. Malformed messages yield a failed ACK, not a panic.
func (e *Engine) Handle(rawMsg []byte, sess session.Session) ([]byte, error) {
	cmd := &controlv1.ControlCommand{}
	if err := proto.Unmarshal(rawMsg, cmd); err != nil {
		svcLog.Warn("command parse error", "error", err)
		return e.ack(sess, "", false, "invalid protobuf message")
	}

	eventID := ""
	if cmd.Header != nil {
		eventID = cmd.Header.EventId
	}

	if !e.limiter.allow() {
		return e.ack(sess, eventID, false, "rate limited")
	}

	switch cmd.Type {
	case controlv1.CommandType_COMMAND_TYPE_DEADMAN_HOLD:
		e.deadman.Reset()

	case controlv1.CommandType_COMMAND_TYPE_DEADMAN_RELEASE:
		// Intentionally do NOT reset — watchdog fires after timeout → SAFE_MODE.

	case controlv1.CommandType_COMMAND_TYPE_EMERGENCY_STOP:
		svcLog.Event(logger.EventEmergencyStop, "EMERGENCY_STOP received → SAFE_MODE",
			"session_id", sess.ID, "vehicle_id", sess.VehicleID, "operator_id", sess.OperatorID)

		// Write to audit store BEFORE state transition (ADR-018)
		if e.auditWriter != nil {
			sys, ctrl, _, _ := e.sm.Get()
			if err := e.auditWriter.WriteSync(audit.SafetyAuditEvent{
				EventID:     ulid.Generate(),
				SessionID:   sess.ID,
				VehicleID:   sess.VehicleID,
				OperatorID:  sess.OperatorID,
				EventType:   logger.EventEmergencyStop,
				Reason:      "operator EMERGENCY_STOP command",
				SystemState: string(sys),
				CtrlState:   string(ctrl),
				Timestamp:   time.Now(),
			}); err != nil {
				svcLog.Error("audit write failed — proceeding to SAFE_MODE", "error", err)
			}
		}

		e.sm.TransitionSystem(statemachine.StateSafeMode)
		e.safetyPub.PublishEvent(safetyservice.SafetyEvent{
			SessionID: sess.ID,
			VehicleID: sess.VehicleID,
			Type:      safetyservice.EventEmergencyStop,
			Reason:    "operator EMERGENCY_STOP command",
			Timestamp: time.Now(),
		})

	case controlv1.CommandType_COMMAND_TYPE_STEER,
		controlv1.CommandType_COMMAND_TYPE_THROTTLE,
		controlv1.CommandType_COMMAND_TYPE_BRAKE,
		controlv1.CommandType_COMMAND_TYPE_SPEED:
		svcLog.Debug("control command",
			"type", cmd.Type, "value", cmd.Value, "session_id", sess.ID)

	default:
		svcLog.Warn("unhandled command type", "type", cmd.Type)
	}

	return e.ack(sess, eventID, true, "")
}

func (e *Engine) ack(sess session.Session, eventID string, success bool, errMsg string) ([]byte, error) {
	if eventID == "" {
		eventID = ulid.Generate()
	}
	ack := &controlv1.ControlAck{
		Header: &commonv1.CorrelationHeader{
			SessionId:  sess.ID,
			EventId:    eventID,
			VehicleId:  sess.VehicleID,
			OperatorId: sess.OperatorID,
			Timestamp:  time.Now().UnixMilli(),
		},
		Success:  success,
		ErrorMsg: errMsg,
	}
	return proto.Marshal(ack)
}

// tokenBucket is a simple token-bucket rate limiter (no external dependency).
type tokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	max      float64
	rate     float64 // tokens per second
	lastTick time.Time
}

func newTokenBucket(ratePerSec float64) *tokenBucket {
	return &tokenBucket{
		tokens:   ratePerSec,
		max:      ratePerSec,
		rate:     ratePerSec,
		lastTick: time.Now(),
	}
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.tokens += now.Sub(b.lastTick).Seconds() * b.rate
	b.lastTick = now
	if b.tokens > b.max {
		b.tokens = b.max
	}
	if b.tokens < 1.0 {
		return false
	}
	b.tokens--
	return true
}
