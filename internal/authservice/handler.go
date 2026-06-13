package authservice

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OperatorRole maps to ADR-011 OPERATOR STATE roles.
type OperatorRole string

const (
	RoleAdmin          OperatorRole = "ADMIN"
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
	secret    []byte
	userStore UserStore
}

func NewHandler(secret string, userStore UserStore) *Handler {
	return &Handler{secret: []byte(secret), userStore: userStore}
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
	Valid   bool         `json:"valid"`
	Subject string       `json:"subject,omitempty"`
	Role    OperatorRole `json:"role,omitempty"`
	Error   string       `json:"error,omitempty"`
}

func (h *Handler) OperatorLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	user, err := h.userStore.Authenticate(r.Context(), req.ID, req.Password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := h.issueToken(user.ID, user.Role, 24*time.Hour)
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

	if req.CurrentToken != "" {
		if _, err := h.parseToken(req.CurrentToken); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	if req.TargetID == "" {
		http.Error(w, "target_id required", http.StatusBadRequest)
		return
	}

	token, err := h.issueToken(req.TargetID, RoleActiveOperator, 1*time.Hour)
	if err != nil {
		http.Error(w, "handover token issuance failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tokenResponse{Token: token})
}

// ─── User Management (ADR-024) ────────────────────────────────────────────────

type userJSON struct {
	ID          string       `json:"id"`
	DisplayName string       `json:"display_name"`
	Role        OperatorRole `json:"role"`
	IsActive    bool         `json:"is_active"`
	CreatedAt   time.Time    `json:"created_at"`
	LastAuthAt  *time.Time   `json:"last_auth_at,omitempty"`
}

func toUserJSON(u User) userJSON {
	return userJSON{
		ID:          u.ID,
		DisplayName: u.DisplayName,
		Role:        u.Role,
		IsActive:    u.IsActive,
		CreatedAt:   u.CreatedAt,
		LastAuthAt:  u.LastAuthAt,
	}
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.userStore.List(r.Context())
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	result := make([]userJSON, len(users))
	for i, u := range users {
		result[i] = toUserJSON(u)
	}
	writeJSON(w, result)
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID          string       `json:"id"`
		DisplayName string       `json:"display_name"`
		Password    string       `json:"password"`
		Role        OperatorRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.ID == "" || req.DisplayName == "" || req.Password == "" {
		http.Error(w, "id, display_name and password required", http.StatusBadRequest)
		return
	}
	if req.Role == "" {
		req.Role = RoleObserver
	}
	if err := h.userStore.Create(r.Context(), req.ID, req.DisplayName, req.Password, req.Role); err != nil {
		http.Error(w, "create failed: "+err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	callerID := h.callerID(r)
	if callerID == id {
		http.Error(w, "cannot delete own account", http.StatusForbidden)
		return
	}
	if err := h.userStore.Delete(r.Context(), id); err != nil {
		http.Error(w, "delete failed: "+err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	var req struct {
		Role OperatorRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Role == "" {
		http.Error(w, "role required", http.StatusBadRequest)
		return
	}
	if err := h.userStore.UpdateRole(r.Context(), id, req.Role); err != nil {
		http.Error(w, "update failed: "+err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "ok", "service": "auth-service"})
}

// ─── Middleware ────────────────────────────────────────────────────────────────

// RequireAdmin returns middleware that enforces a valid JWT with role=ADMIN.
func (h *Handler) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := h.claimsFromRequest(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if claims.Role != RoleAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

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

func (h *Handler) claimsFromRequest(r *http.Request) (*Claims, error) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("missing token")
	}
	return h.parseToken(strings.TrimPrefix(authHeader, "Bearer "))
}

func (h *Handler) callerID(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	claims, err := h.parseToken(strings.TrimPrefix(authHeader, "Bearer "))
	if err != nil {
		return ""
	}
	return claims.Subject
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
