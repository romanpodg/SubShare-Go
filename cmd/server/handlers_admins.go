package main

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/model"
	adminpassword "github.com/romanpodg/SubShare-Go/internal/security/password"
)

// --- Admins Management API (Super Admin only) ---

func (a *App) apiListAdmins(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`SELECT id, username, role, created_at FROM admins ORDER BY id ASC`)
	if err != nil {
		log.Printf("apiListAdmins: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to query admins")
		return
	}
	defer rows.Close()

	var admins []model.Admin
	for rows.Next() {
		var adm model.Admin
		if err := rows.Scan(&adm.ID, &adm.Username, &adm.Role, &adm.CreatedAt); err != nil {
			log.Printf("apiListAdmins scan: %v", err)
			httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to scan admins")
			return
		}
		admins = append(admins, adm)
	}

	if admins == nil {
		admins = []model.Admin{}
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"admins": admins})
}

func (a *App) apiCreateAdmin(w http.ResponseWriter, r *http.Request) {
	var req model.CreateAdminRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	username := strings.TrimSpace(req.Username)
	role, roleOK := normalizeAdminRole(req.Role)

	if username == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "username is required")
		return
	}
	if !roleOK {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid role")
		return
	}

	// Check if username already exists
	var exists bool
	err := a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM admins WHERE username = ?)`, username).Scan(&exists)
	if err != nil {
		log.Printf("apiCreateAdmin check exists: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal database error")
		return
	}
	if exists {
		httpapi.WriteError(w, r, http.StatusConflict, "username is already taken")
		return
	}

	passwordHash, err := a.passwordHasher().Hash(req.Password)
	if err != nil {
		if adminpassword.IsPolicyError(err) {
			httpapi.WriteError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("apiCreateAdmin: password hashing failed")
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to hash password")
		return
	}

	result, err := a.db.Exec(`INSERT INTO admins (username, password_hash, role) VALUES (?, ?, ?)`, username, passwordHash, role)
	if err != nil {
		log.Printf("apiCreateAdmin insert: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to create admin")
		return
	}
	adminID, _ := result.LastInsertId()
	a.recordAuditEvent(r, "admin.create", "admin", strconv.FormatInt(adminID, 10), map[string]any{"username": username, "role": role})

	httpapi.WriteMessage(w, "administrator created successfully")
}

func (a *App) apiUpdateAdmin(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid administrator id")
		return
	}

	var req model.UpdateAdminRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	role := strings.TrimSpace(req.Role)
	passwordValue := req.Password

	session, _, _ := a.adminSessionFromRequest(r)

	// sources.Fetch current admin info
	var currentUsername string
	var currentRole string
	err = a.db.QueryRow(`SELECT username, role FROM admins WHERE id = ?`, id).Scan(&currentUsername, &currentRole)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpapi.WriteError(w, r, http.StatusNotFound, "administrator not found")
			return
		}
		log.Printf("apiUpdateAdmin query: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "database error")
		return
	}

	// Update password if provided
	if passwordValue != "" && !a.updateAdminPassword(w, r, id, passwordValue) {
		return
	}

	// Update role if provided
	if role != "" {
		normalizedRole, ok := a.updateAdminRole(w, r, adminRoleChange{
			ID:             id,
			SessionAdminID: session.AdminID,
			CurrentRole:    currentRole,
			NewRole:        role,
		})
		if !ok {
			return
		}
		role = normalizedRole
	}

	a.recordAuditEvent(r, "admin.update", "admin", strconv.FormatInt(id, 10), map[string]any{
		"role":             role,
		"password_changed": passwordValue != "",
	})
	httpapi.WriteMessage(w, "administrator updated successfully")
}

// updateAdminPassword hashes and stores the new password and invalidates the
// admin's other sessions. It returns false after writing an error response.
func (a *App) updateAdminPassword(w http.ResponseWriter, r *http.Request, id int64, passwordValue string) bool {
	passwordHash, err := a.passwordHasher().Hash(passwordValue)
	if err != nil {
		if adminpassword.IsPolicyError(err) {
			httpapi.WriteError(w, r, http.StatusBadRequest, err.Error())
			return false
		}
		log.Printf("apiUpdateAdmin: password hashing failed")
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to hash password")
		return false
	}
	_, err = a.db.Exec(`UPDATE admins SET password_hash = ? WHERE id = ?`, passwordHash, id)
	if err != nil {
		log.Printf("apiUpdateAdmin password update: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update password")
		return false
	}
	// Invalidate all active sessions for this admin since password changed, except the current one
	cookie, err := r.Cookie(model.AdminSessionCookieName)
	if err == nil && cookie.Value != "" {
		_, _ = a.db.Exec(`DELETE FROM admin_sessions WHERE admin_id = ? AND id != ?`, id, hashSessionID(cookie.Value))
	} else {
		_, _ = a.db.Exec(`DELETE FROM admin_sessions WHERE admin_id = ?`, id)
	}
	return true
}

// adminRoleChange describes a requested role update for one administrator.
type adminRoleChange struct {
	ID             int64
	SessionAdminID int64
	CurrentRole    string
	NewRole        string
}

// updateAdminRole validates and stores the new role and invalidates the
// admin's sessions. It returns the normalized role, or false after writing an
// error response.
func (a *App) updateAdminRole(w http.ResponseWriter, r *http.Request, change adminRoleChange) (string, bool) {
	id, sessionAdminID, currentRole, role := change.ID, change.SessionAdminID, change.CurrentRole, change.NewRole
	normalizedRole, roleOK := normalizeAdminRole(role)
	if !roleOK {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid role")
		return "", false
	}
	role = normalizedRole

	// Prevent changing own role
	if id == sessionAdminID {
		httpapi.WriteError(w, r, http.StatusBadRequest, "you cannot change your own role")
		return "", false
	}

	// Prevent changing role of the last owner
	if isOwnerRole(currentRole) && !isOwnerRole(role) {
		var superAdminCount int
		err := a.db.QueryRow(`SELECT COUNT(*) FROM admins WHERE role IN ('owner', 'super_admin')`).Scan(&superAdminCount)
		if err != nil {
			log.Printf("apiUpdateAdmin count super admins: %v", err)
			httpapi.WriteError(w, r, http.StatusInternalServerError, "database error")
			return "", false
		}
		if superAdminCount <= 1 {
			httpapi.WriteError(w, r, http.StatusBadRequest, "cannot demote the only remaining super admin")
			return "", false
		}
	}

	_, err := a.db.Exec(`UPDATE admins SET role = ? WHERE id = ?`, role, id)
	if err != nil {
		log.Printf("apiUpdateAdmin role update: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update role")
		return "", false
	}

	// Invalidate all sessions for the updated admin since role changed
	_, _ = a.db.Exec(`DELETE FROM admin_sessions WHERE admin_id = ?`, id)
	return role, true
}

func (a *App) apiDeleteAdmin(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid administrator id")
		return
	}

	session, _, _ := a.adminSessionFromRequest(r)

	// Prevent deleting oneself
	if id == session.AdminID {
		httpapi.WriteError(w, r, http.StatusBadRequest, "you cannot delete your own account")
		return
	}

	// sources.Fetch admin to delete
	var role string
	err = a.db.QueryRow(`SELECT role FROM admins WHERE id = ?`, id).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpapi.WriteError(w, r, http.StatusNotFound, "administrator not found")
			return
		}
		log.Printf("apiDeleteAdmin query: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "database error")
		return
	}

	// Prevent deleting the last owner
	if isOwnerRole(role) {
		var superAdminCount int
		err = a.db.QueryRow(`SELECT COUNT(*) FROM admins WHERE role IN ('owner', 'super_admin')`).Scan(&superAdminCount)
		if err != nil {
			log.Printf("apiDeleteAdmin count super admins: %v", err)
			httpapi.WriteError(w, r, http.StatusInternalServerError, "database error")
			return
		}
		if superAdminCount <= 1 {
			httpapi.WriteError(w, r, http.StatusBadRequest, "cannot delete the only remaining super admin")
			return
		}
	}

	// Delete sessions first
	_, err = a.db.Exec(`DELETE FROM admin_sessions WHERE admin_id = ?`, id)
	if err != nil {
		log.Printf("apiDeleteAdmin delete sessions: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to delete sessions")
		return
	}

	// Delete admin
	res, err := a.db.Exec(`DELETE FROM admins WHERE id = ?`, id)
	if err != nil {
		log.Printf("apiDeleteAdmin delete admin: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to delete admin")
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		httpapi.WriteError(w, r, http.StatusNotFound, "administrator not found")
		return
	}

	a.recordAuditEvent(r, "admin.delete", "admin", strconv.FormatInt(id, 10), nil)
	httpapi.WriteMessage(w, "administrator deleted successfully")
}
