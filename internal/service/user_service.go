package service

import (
	"context"
	"fmt"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"
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
	return nil, fmt.Errorf("user service Create not implemented")
}

func (s *UserService) GetByID(ctx context.Context, id uint) (*models.User, error) {
	return nil, fmt.Errorf("user service GetByID not implemented")
}

func (s *UserService) GetCurrent(ctx context.Context) (*models.User, error) {
	return nil, fmt.Errorf("user service GetCurrent not implemented")
}

func (s *UserService) UpdateCurrentProfile(ctx context.Context, input UpdateProfileInput) (*models.User, error) {
	return nil, fmt.Errorf("user service UpdateCurrentProfile not implemented")
}

func (s *UserService) LoginOrCreate(ctx context.Context, authUID, openID string) (*models.User, error) {
	return nil, fmt.Errorf("user service LoginOrCreate not implemented")
}
