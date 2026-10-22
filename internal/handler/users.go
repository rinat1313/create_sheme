package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"create_sheme/internal/service"
)

func UserContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(withUserContext(r)))
	})
}

func (a *API) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listUsers(w, r)
	case http.MethodPost:
		a.createUser(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		slog.Info("listUsers done", "duration_ms", time.Since(start).Milliseconds())
	}()
	users, err := a.users.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, users)
}

func (a *API) createUser(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		slog.Info("createUser done", "duration_ms", time.Since(start).Milliseconds())
	}()
	var body struct {
		Login       string `json:"login"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := a.users.CreateUser(r.Context(), body.Login, body.DisplayName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, user)
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/api/v1/me"):]
	if path == "" || path == "/" {
		http.NotFound(w, r)
		return
	}
	if path == "/solutions" {
		a.listMySolutions(w, r)
		return
	}
	if path == "/settings" {
		a.getMySettings(w, r)
		return
	}
	http.NotFound(w, r)
}

func (a *API) listMySolutions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := requireUserID(w, r); !ok {
		return
	}
	items, err := a.svc.ListSolutionsForUser(r.Context(), service.UserIDFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var out []map[string]any
	for _, it := range items {
		row := map[string]any{
			"id": it.ID.String(), "name": it.Name, "company": it.Company,
			"is_dirty": it.IsDirty, "import_status": it.ImportStatus,
			"created_at": it.CreatedAt.Format(time.RFC3339),
		}
		if it.ImportError != "" {
			row["import_error"] = it.ImportError
		}
		attachImportProgressFields(row, a.svc, it.ID, it.ImportStatus)
		out = append(out, row)
	}
	writeJSON(w, out)
}

func (a *API) getMySettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := requireUserID(w, r); !ok {
		return
	}
	settings, err := a.svc.GetUserSettings(r.Context(), service.UserIDFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, settings)
}
