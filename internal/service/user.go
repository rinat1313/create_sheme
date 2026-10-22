package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"create_sheme/internal/repository"

	"github.com/google/uuid"
)

type UserService struct {
	store *repository.Store
}

func NewUserService(store *repository.Store) *UserService {
	return &UserService{store: store}
}

type UserInfo struct {
	ID          uuid.UUID `json:"id"`
	Login       string    `json:"login"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

func (u *UserService) ListUsers(ctx context.Context) ([]UserInfo, error) {
	start := time.Now()
	defer func() {
		slog.Info("ListUsers done", "duration_ms", time.Since(start).Milliseconds())
	}()

	rows, err := u.store.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]UserInfo, len(rows))
	for i, r := range rows {
		result[i] = UserInfo{ID: r.ID, Login: r.Login, DisplayName: r.DisplayName, CreatedAt: r.CreatedAt}
	}
	return result, nil
}

func (u *UserService) CreateUser(ctx context.Context, login, displayName string) (UserInfo, error) {
	start := time.Now()
	defer func() {
		slog.Info("CreateUser done", "login", login, "duration_ms", time.Since(start).Milliseconds())
	}()

	if login == "" {
		return UserInfo{}, fmt.Errorf("login required")
	}
	if displayName == "" {
		displayName = login
	}
	id := uuid.New()
	row := repository.UserRow{ID: id, Login: login, DisplayName: displayName}
	if err := u.store.InsertUser(ctx, row); err != nil {
		return UserInfo{}, err
	}
	if err := u.store.SeedUserSettingsFromDefault(ctx, id); err != nil {
		return UserInfo{}, err
	}
	return UserInfo{ID: id, Login: login, DisplayName: displayName, CreatedAt: time.Now()}, nil
}

func (u *UserService) GetUser(ctx context.Context, id uuid.UUID) (UserInfo, error) {
	row, err := u.store.GetUser(ctx, id)
	if err != nil {
		return UserInfo{}, err
	}
	return UserInfo{ID: row.ID, Login: row.Login, DisplayName: row.DisplayName, CreatedAt: row.CreatedAt}, nil
}

type UserSettings struct {
	DisplaySettings []repository.DisplaySetting `json:"display_settings"`
	ComplianceRules []repository.ComplianceRule `json:"compliance_rules"`
	TagEditRules    []repository.TagEditRule    `json:"tag_edit_rules"`
}

func (s *SolutionService) ListSolutionsForUser(ctx context.Context, userID uuid.UUID) ([]repository.SolutionSummary, error) {
	start := time.Now()
	defer func() {
		slog.Info("ListSolutionsForUser done", "user_id", userID, "duration_ms", time.Since(start).Milliseconds())
	}()
	return s.store.ListSolutionsForUser(ctx, userID)
}

func (s *SolutionService) GetUserSettings(ctx context.Context, userID uuid.UUID) (UserSettings, error) {
	start := time.Now()
	defer func() {
		slog.Info("GetUserSettings done", "user_id", userID, "duration_ms", time.Since(start).Milliseconds())
	}()

	display, err := s.store.ListDisplaySettings(ctx, userID)
	if err != nil {
		return UserSettings{}, err
	}
	compliance, err := s.store.ListComplianceRules(ctx, userID, nil)
	if err != nil {
		return UserSettings{}, err
	}
	tagEdit, err := s.store.ListTagEditRules(ctx, userID)
	if err != nil {
		return UserSettings{}, err
	}
	return UserSettings{
		DisplaySettings: display,
		ComplianceRules: compliance,
		TagEditRules:    tagEdit,
	}, nil
}

func (s *SolutionService) DeleteAllSolutionsForUser(ctx context.Context, userID uuid.UUID) (int, error) {
	start := time.Now()
	var err error
	var count int
	defer func() {
		slog.Info("DeleteAllSolutionsForUser done", "user_id", userID, "count", count, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	items, listErr := s.store.ListSolutionsForUser(ctx, userID)
	if listErr != nil {
		err = listErr
		return 0, err
	}
	count, err = s.store.DeleteAllSolutionsForUser(ctx, userID)
	if err == nil {
		for _, item := range items {
			s.BumpSolutionGen(ctx, item.ID)
		}
	}
	return count, err
}

func (s *SolutionService) EnsureUserAccess(ctx context.Context, userID, solutionID uuid.UUID) error {
	ok, err := s.store.UserHasSolution(ctx, userID, solutionID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("access denied")
	}
	return nil
}

func (s *SolutionService) LinkSolutionToUser(ctx context.Context, userID, solutionID uuid.UUID) error {
	return s.store.LinkSolutionToUser(ctx, userID, solutionID, "owner")
}
