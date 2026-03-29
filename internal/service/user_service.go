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
