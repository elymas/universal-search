package rbac

import (
	"encoding/json"
	"net/http"
)

// AdminHandlers provides HTTP handlers for RBAC admin operations.
// REQ-AUTH2-009, REQ-AUTH2-010.
type AdminHandlers struct {
	enforcer *Enforcer
}

// NewAdminHandlers creates admin handlers with the given enforcer.
func NewAdminHandlers(ef *Enforcer) *AdminHandlers {
	return &AdminHandlers{enforcer: ef}
}

// ReloadHandler handles POST /admin/rbac/reload.
// REQ-AUTH2-009: Reloads policy from PG. Failure preserves existing enforcer.
func (h *AdminHandlers) ReloadHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.enforcer.LoadPolicy(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":  "reload_failed",
			"reason": err.Error(),
		})
		return
	}

	count := h.enforcer.GetPolicyCount()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "reloaded",
		"policy_count": count,
	})
}

// AddMemberRequest is the request body for POST /admin/members.
type AddMemberRequest struct {
	UserID string `json:"user_id"`
	TeamID string `json:"team_id"`
	Role   string `json:"role"`
}

// AddMemberHandler handles POST /admin/members.
// REQ-AUTH2-010: Adds role assignment for user in domain.
func (h *AdminHandlers) AddMemberHandler(w http.ResponseWriter, r *http.Request) {
	var req AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_body"})
		return
	}

	// Validate role.
	if !ValidRoles[Role(req.Role)] {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_role"})
		return
	}

	if err := h.enforcer.AddRoleForUserInDomain(req.UserID, req.Role, req.TeamID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if err := h.enforcer.SavePolicy(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"user_id": req.UserID,
		"team_id": req.TeamID,
		"role":    req.Role,
	})
}

// RemoveMemberHandler handles DELETE /admin/members.
// REQ-AUTH2-010: Removes role assignment.
func (h *AdminHandlers) RemoveMemberHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	teamID := r.URL.Query().Get("team_id")
	role := r.URL.Query().Get("role")

	if userID == "" || teamID == "" || role == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing_parameters"})
		return
	}

	if !ValidRoles[Role(role)] {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_role"})
		return
	}

	if err := h.enforcer.DeleteRoleForUserInDomain(userID, role, teamID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if err := h.enforcer.SavePolicy(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListMembersHandler handles GET /admin/members.
// REQ-AUTH2-010: Lists members with roles for a team.
func (h *AdminHandlers) ListMembersHandler(w http.ResponseWriter, r *http.Request) {
	teamID := r.URL.Query().Get("team_id")
	if teamID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "team_id_required"})
		return
	}

	// Collect all users across known roles.
	type Member struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}

	var members []Member
	roles := []string{string(RoleAdmin), string(RoleMember), string(RoleObserver)}
	for _, role := range roles {
		users := h.enforcer.GetUsersForRoleInDomain(role, teamID)
		for _, u := range users {
			members = append(members, Member{UserID: u, Role: role})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"team_id": teamID,
		"members": members,
	})
}
