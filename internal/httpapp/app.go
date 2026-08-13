package httpapp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"pscpt/internal/auth"
	"pscpt/internal/config"
	"pscpt/internal/psmonth"
	"pscpt/internal/uploads"
)

type App struct {
	DB  *sql.DB
	Cfg config.Config
}

type user struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (a App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", a.health)
	mux.HandleFunc("/api/auth/register", a.register)
	mux.HandleFunc("/api/auth/login", a.login)
	mux.HandleFunc("/api/auth/logout", a.logout)
	mux.HandleFunc("/api/auth/me", a.me)
	mux.HandleFunc("/api/auth/forgot-password", a.forgotPassword)
	mux.HandleFunc("/api/auth/reset-password", a.resetPassword)
	mux.HandleFunc("/api/auth/change-password", a.changePassword)
	mux.HandleFunc("/api/dashboard/summary", a.requireAuth(a.summary))
	mux.HandleFunc("/api/uploads/manual", a.requireAuth(a.uploadManual))
	mux.HandleFunc("/api/uploads/manual/list", a.requireAuth(a.listManualUploads))
	mux.HandleFunc("/api/analysis/preview", a.requireAuth(a.preview))
	mux.HandleFunc("/api/ps-month/snapshots", a.requireAuth(a.listPSMonthSnapshots))
	mux.HandleFunc("/api/ps-month/snapshots/create", a.requireAuth(a.createPSMonthSnapshot))
	mux.HandleFunc("/api/ps-month/rows", a.requireAuth(a.listPSMonthRows))
	mux.Handle("/", spaFileServer("web/dist"))
	return securityHeaders(mux)
}

func (a App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a App) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var in struct{ Name, Email, Password string }
	if !decodeJSON(w, r, &in) {
		return
	}
	name := strings.TrimSpace(in.Name)
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if name == "" || !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "name and valid email required")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := a.DB.Exec(`INSERT INTO users (name,email,password_hash) VALUES (?,?,?)`, name, email, hash)
	if err != nil {
		writeError(w, http.StatusConflict, "email already registered")
		return
	}
	id, _ := res.LastInsertId()
	u := user{ID: id, Name: name, Email: email, Role: "user"}
	a.setSession(w, u.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"user": u})
}

func (a App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var in struct{ Email, Password string }
	if !decodeJSON(w, r, &in) {
		return
	}
	u, hash, err := a.userByEmail(strings.ToLower(strings.TrimSpace(in.Email)))
	if err != nil || !auth.CheckPassword(hash, in.Password) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	a.setSession(w, u.ID)
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

func (a App) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "pscpt_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a App) me(w http.ResponseWriter, r *http.Request) {
	u, err := a.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

func (a App) forgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var in struct{ Email string }
	if !decodeJSON(w, r, &in) {
		return
	}
	u, _, err := a.userByEmail(strings.ToLower(strings.TrimSpace(in.Email)))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	token, err := auth.NewToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed")
		return
	}
	_, _ = a.DB.Exec(`INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES (?,?,?)`, u.ID, auth.TokenHash(token), time.Now().Add(30*time.Minute))
	// Dev/local-first delivery: return reset link in JSON until SMTP integration exists.
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reset_link": fmt.Sprintf("%s/reset-password?token=%s", a.Cfg.AppURL, token)})
}

func (a App) resetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var in struct{ Token, Password string }
	if !decodeJSON(w, r, &in) {
		return
	}
	var tokenID, userID int64
	err := a.DB.QueryRow(`SELECT id,user_id FROM password_reset_tokens WHERE token_hash=? AND used_at IS NULL AND expires_at > NOW()`, auth.TokenHash(in.Token)).Scan(&tokenID, &userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err = a.DB.Exec(`UPDATE users SET password_hash=? WHERE id=?; UPDATE password_reset_tokens SET used_at=NOW() WHERE id=?`, hash, userID, tokenID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "password reset failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a App) changePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	u, err := a.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var in struct{ CurrentPassword, NewPassword string }
	if !decodeJSON(w, r, &in) {
		return
	}
	_, hash, err := a.userByEmail(u.Email)
	if err != nil || !auth.CheckPassword(hash, in.CurrentPassword) {
		writeError(w, http.StatusUnauthorized, "current password invalid")
		return
	}
	newHash, err := auth.HashPassword(in.NewPassword)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err = a.DB.Exec(`UPDATE users SET password_hash=? WHERE id=?`, newHash, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "change password failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a App) createPSMonthSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	u, err := a.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var in struct{ Period string }
	if !decodeJSON(w, r, &in) {
		return
	}
	if len(in.Period) != 7 || in.Period[4] != '-' {
		writeError(w, http.StatusBadRequest, "period must use YYYY-MM")
		return
	}
	id, err := psmonth.CreateSnapshot(a.DB, in.Period, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create snapshot failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "period": in.Period})
}

func (a App) listPSMonthSnapshots(w http.ResponseWriter, r *http.Request) {
	items, err := psmonth.ListSnapshots(a.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list snapshots failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshots": items})
}

func (a App) listPSMonthRows(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("snapshot_id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "snapshot_id required")
		return
	}
	items, err := psmonth.ListRows(a.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list rows failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": items})
}

func (a App) uploadManual(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	u, err := a.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	kind := r.FormValue("type")
	if kind != "individual_factor" && kind != "leadership" {
		writeError(w, http.StatusBadRequest, "type must be individual_factor or leadership")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file required")
		return
	}
	defer file.Close()
	parsed, err := uploads.Parse(file, header)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := uploads.Save(a.DB, kind, header.Filename, u.ID, parsed)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "save upload failed")
		return
	}
	preview := parsed.Rows
	if len(preview) > 5 {
		preview = preview[:5]
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "type": kind, "filename": header.Filename, "columns": parsed.Columns, "row_count": parsed.Count, "preview": preview})
}

func (a App) listManualUploads(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(`SELECT id, upload_type, filename, row_count, created_at FROM manual_uploads ORDER BY id DESC LIMIT 50`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list uploads failed")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var kind, filename string
		var count int
		var created time.Time
		if err := rows.Scan(&id, &kind, &filename, &count, &created); err != nil {
			writeError(w, http.StatusInternalServerError, "scan uploads failed")
			return
		}
		items = append(items, map[string]any{"id": id, "type": kind, "filename": filename, "row_count": count, "created_at": created})
	}
	writeJSON(w, http.StatusOK, map[string]any{"uploads": items})
}

func (a App) summary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"cards": []map[string]any{{"label": "PS Month Rows", "value": "0", "tone": "ink"}, {"label": "Manual Uploads", "value": "0", "tone": "amber"}, {"label": "Synced Sources", "value": "4", "tone": "green"}, {"label": "Alerts", "value": "0", "tone": "red"}}})
}

func (a App) preview(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"grain": "PS-month", "sources": []string{"Talenta", "Moodle", "Emica Lead", "Emica Sales Order", "Manual Excel"}, "next": "upload manual factors, then sync sources"})
}

func (a App) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := a.currentUser(r); err != nil {
			writeError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		next(w, r)
	}
}

func (a App) currentUser(r *http.Request) (user, error) {
	c, err := r.Cookie("pscpt_session")
	if err != nil {
		return user{}, err
	}
	id, err := strconv.ParseInt(c.Value, 10, 64)
	if err != nil {
		return user{}, err
	}
	var u user
	err = a.DB.QueryRow(`SELECT id,name,email,role FROM users WHERE id=?`, id).Scan(&u.ID, &u.Name, &u.Email, &u.Role)
	return u, err
}
func (a App) userByEmail(email string) (user, string, error) {
	var u user
	var hash string
	err := a.DB.QueryRow(`SELECT id,name,email,role,password_hash FROM users WHERE email=?`, email).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &hash)
	return u, hash, err
}
func (a App) setSession(w http.ResponseWriter, userID int64) {
	http.SetCookie(w, &http.Cookie{Name: "pscpt_session", Value: strconv.FormatInt(userID, 10), Path: "/", MaxAge: 86400 * 14, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return false
	}
	return true
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

type spa struct{ root string }

func spaFileServer(root string) http.Handler { return spa{root: root} }
func (s spa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	path := filepath.Join(s.root, clean)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.root, "index.html"))
}
