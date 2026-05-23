package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"
	"github.com/QSCTech/SRTP-Backend/pkg/utils"
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
	authUID = utils.NormalizeWhitespace(authUID)
	if strings.TrimSpace(authUID) == "" {
		return nil, fmt.Errorf("auth_uid is required")
	}
	user := &models.User{AuthUID: authUID}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) GetByID(ctx context.Context, id uint) (*models.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return user, nil
}

func (s *UserService) GetCurrent(ctx context.Context) (*models.User, error) {
	user, err := s.repo.GetFirst(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return user, nil
}

func (s *UserService) UpdateCurrentProfile(ctx context.Context, input UpdateProfileInput) (*models.User, error) {
	user, err := s.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	if input.Nickname != nil {
		user.Nickname = utils.NormalizeWhitespace(*input.Nickname)
	}
	if input.AvatarURL != nil {
		user.AvatarURL = strings.TrimSpace(*input.AvatarURL)
	}
	if input.Gender != nil {
		user.Gender = strings.TrimSpace(*input.Gender)
	}
	if input.Bio != nil {
		user.Bio = utils.NormalizeWhitespace(*input.Bio)
	}
	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) LoginOrCreate(ctx context.Context, authUID, openID string) (*models.User, error) {
	user, err := s.repo.GetByAuthUID(ctx, authUID)
	if err == nil {
		return user, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	user = &models.User{
		AuthUID: authUID,
		OpenID:  openID,
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}
