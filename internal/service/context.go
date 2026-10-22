package service

import (
	"context"

	"create_sheme/internal/repository"

	"github.com/google/uuid"
)

type contextKey string

const UserIDKey contextKey = "userID"

func ContextWithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

func UserIDFromContext(ctx context.Context) uuid.UUID {
	v := ctx.Value(UserIDKey)
	if v == nil {
		return repository.DefaultUserID
	}
	id, ok := v.(uuid.UUID)
	if !ok {
		return repository.DefaultUserID
	}
	return id
}
