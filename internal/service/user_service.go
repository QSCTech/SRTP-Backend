package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/serein6174/SRTP-backend/internal/repository"
	"github.com/serein6174/SRTP-backend/models"
	"github.com/serein6174/SRTP-backend/pkg/utils"
	"gorm.io/gorm"
)

type UserService struct {
	repo *repository.UserRepository
}

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Create(ctx context.Context, name string) (*models.User, error) {
	name = utils.NormalizeWhitespace(name)
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("name is required")
	}

	user := &models.User{Name: name}
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
