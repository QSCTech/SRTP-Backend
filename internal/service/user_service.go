package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"
	"gorm.io/gorm"
)

type UserService struct {
	repo *repository.UserRepository
}

type UpdateProfileInput struct {
	Nickname  *string
	AvatarURL *string
	Gender    *string
	Bio       *string
}

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// Create registers a new user with the given authUID.
// Returns the existing user if authUID is already taken.
func (s *UserService) Create(ctx context.Context, authUID string) (*models.User, error) {
	authUID = strings.TrimSpace(authUID)
	if authUID == "" {
		return nil, fmt.Errorf("auth_uid is required")
	}
	existing, err := s.repo.GetByAuthUID(ctx, authUID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("check existing user: %w", err)
	}
	user := &models.User{
		AuthUID:       authUID,
		ProfileStatus: "approved",
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// GetByID returns the user with the given primary key.
func (s *UserService) GetByID(ctx context.Context, id uint) (*models.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// GetCurrent returns the acting user for this request.
// Dev fallback: returns the first user in DB until real auth middleware is wired up.
func (s *UserService) GetCurrent(ctx context.Context) (*models.User, error) {
	if uid, ok := ctx.Value(ctxKeyUserID{}).(uint); ok && uid != 0 {
		return s.repo.GetByID(ctx, uid)
	}
	user, err := s.repo.GetFirst(ctx)
	if err != nil {
		return nil, fmt.Errorf("no users found")
	}
	return user, nil
}

// UpdateCurrentProfile applies whichever profile fields are non-nil.
func (s *UserService) UpdateCurrentProfile(ctx context.Context, input UpdateProfileInput) (*models.User, error) {
	user, err := s.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	if input.Nickname != nil {
		user.Nickname = *input.Nickname
	}
	if input.AvatarURL != nil {
		user.AvatarURL = *input.AvatarURL
	}
	if input.Gender != nil {
		user.Gender = *input.Gender
	}
	if input.Bio != nil {
		user.Bio = *input.Bio
	}
	if err := s.repo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	return user, nil
}

// LoginOrCreate finds a user by authUID or creates a new one.
func (s *UserService) LoginOrCreate(ctx context.Context, authUID, openID string) (*models.User, error) {
	authUID = strings.TrimSpace(authUID)
	if authUID == "" {
		return nil, fmt.Errorf("auth_uid is required")
	}
	user, err := s.repo.GetByAuthUID(ctx, authUID)
	if err == nil {
		if openID != "" && user.OpenID != openID {
			user.OpenID = openID
			if updateErr := s.repo.Update(ctx, user); updateErr != nil {
				return nil, fmt.Errorf("update open_id: %w", updateErr)
			}
		}
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	user = &models.User{
		AuthUID:       authUID,
		OpenID:        openID,
		ProfileStatus: "approved",
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// ctxKeyUserID is the context key used by auth middleware to pass the current user ID.
type ctxKeyUserID struct{}
