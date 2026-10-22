package handler

import (
	"context"
	"net/http"

	"create_sheme/internal/repository"
	"create_sheme/internal/service"

	"github.com/google/uuid"
)

func userIDFromRequest(r *http.Request) (uuid.UUID, bool) {
	raw := r.Header.Get("X-User-Id")
	if raw == "" {
		raw = r.URL.Query().Get("user_id")
	}
	if raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func withUserContext(r *http.Request) context.Context {
	if id, ok := userIDFromRequest(r); ok {
		return service.ContextWithUserID(r.Context(), id)
	}
	return service.ContextWithUserID(r.Context(), repository.DefaultUserID)
}

func requireUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := userIDFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "X-User-Id required")
		return uuid.Nil, false
	}
	return id, true
}
