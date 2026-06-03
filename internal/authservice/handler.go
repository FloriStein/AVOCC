package authservice

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OperatorRole maps to ADR-011 OPERATOR STATE roles.
type OperatorRole string

const (
	RoleActiveOperator OperatorRole = "ACTIVE_OPERATOR"
	RoleObserver       OperatorRole = "OBSERVER"
	RoleStandby        OperatorRole = "STANDBY"
	RoleVehicle        OperatorRole = "VEHICLE"
)

// Claims extends jwt.RegisteredClaims with system-specific fields.
// JWT = Identity only — no Session-ID (ADR-016).
type Claims struct {
	jwt.RegisteredClaims
	Role OperatorRole `json:"role"`
}

type Handler struct {
	secret []byte
}

func NewHandler(secret string) *Handler {
	return &Handler{secret: []byte(secret)}
}

type loginRequest struct {
	ID       string `json:"id"`
	Password string `json:"password"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

type validateRequest struct {
	Token string `json:"token"`
}

type validateResponse struct {
	Valid      bool         `json:"valid"`
	Subject    string       `json:"subject,omitempty"`
	Role       OperatorRole `json:"role,omitempty"`
	Error      string       `json:"error,omitempty"`
}

func (h *Handler) OperatorLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Sprint 1 skeleton: accept any credentials, return OBSERVER role.
	// Production: validate against user store.
	token, err := h.issueToken(req.ID, RoleObserver, 24*time.Hour)
	if err != nil {
		http.Error(w, "token issuance failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tokenResponse{Token: token})
}

func (h *Handler) VehicleRegister(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	token, err := h.issueToken(req.ID, RoleVehicle, 168*time.Hour)
	if err != nil {
		http.Error(w, "token issuance failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tokenResponse{Token: token})
}

func (h *Handler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	claims, err := h.parseToken(req.Token)
	if err != nil {
		writeJSON(w, validateResponse{Valid: false, Error: err.Error()})
		return
	}
	writeJSON(w, validateResponse{
		Valid:   true,
		Subject: claims.Subject,
		Role:    claims.Role,
	})
}

func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	claims, err := h.parseToken(req.Token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	token, err := h.issueToken(claims.Subject, claims.Role, 24*time.Hour)
	if err != nil {
		http.Error(w, "token refresh failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tokenResponse{Token: token})
}

func (h *Handler) HandoverToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentToken string `json:"current_token"`
		TargetID     string `json:"target_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if _, err := h.parseToken(req.CurrentToken); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Issue a short-lived ACTIVE_OPERATOR token for the target operator.
	token, err := h.issueToken(req.TargetID, RoleActiveOperator, 1*time.Hour)
	if err != nil {
		http.Error(w, "handover token issuance failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tokenResponse{Token: token})
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "ok", "service": "auth-service"})
}

func (h *Handler) issueToken(subject string, role OperatorRole, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Role: role,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(h.secret)
}

func (h *Handler) parseToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(_ *jwt.Token) (any, error) {
		return h.secret, nil
	})
	return claims, err
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
