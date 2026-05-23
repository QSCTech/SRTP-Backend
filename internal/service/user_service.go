package service

import (
	"context"
	"errors"
	"fmt"

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

func (s *UserService) Create(ctx context.Context, authUID string) (*models.User, error) {
	user := &models.User{
		AuthUID:       authUID,
		ProfileStatus: "pending",
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return user, nil
}

func (s *UserService) GetByID(ctx context.Context, id uint) (*models.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return user, nil
}

func (s *UserService) GetCurrent(ctx context.Context) (*models.User, error) {
	userID, ok := ctx.Value("userID").(uint)
	if !ok || userID == 0 {
		return nil, fmt.Errorf("user not authenticated")
	}
	return s.GetByID(ctx, userID)
}

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
		return nil, fmt.Errorf("failed to update user profile: %w", err)
	}
	return user, nil
}

func (s *UserService) LoginOrCreate(ctx context.Context, authUID, openID string) (*models.User, error) {
	existing, err := s.repo.GetByAuthUID(ctx, authUID)
	if err == nil {
		if openID != "" && existing.OpenID != openID {
			existing.OpenID = openID
			_ = s.repo.Update(ctx, existing)
		}
		return existing, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	user := &models.User{
		AuthUID:       authUID,
		OpenID:        openID,
		ProfileStatus: "pending",
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user during login: %w", err)
	}
	return user, nil
}