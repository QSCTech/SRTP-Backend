package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QSCTech/SRTP-Backend/internal/middleware"
	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"
	"gorm.io/gorm"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrUserNotFound = errors.New("user not found")
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
	authUID = strings.TrimSpace(authUID)
	if authUID == "" {
		return nil, fmt.Errorf("auth uid is required")
	}
	if _, err := s.repo.GetByAuthUID(ctx, authUID); err == nil {
		return nil, fmt.Errorf("auth uid already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	user := &models.User{AuthUID: authUID, ProfileStatus: "pending"}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) GetByID(ctx context.Context, id uint) (*models.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) GetCurrent(ctx context.Context) (*models.User, error) {
	authUID, _ := ctx.Value(middleware.AuthUIDKey).(string)
	authUID = strings.TrimSpace(authUID)
	if authUID == "" {
		return nil, ErrUnauthorized
	}
	user, err := s.repo.GetByAuthUID(ctx, authUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
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
	user.ProfileStatus = "pending_review"

	audit := &models.UserProfileAudit{
		UserID:            user.ID,
		SubmittedNickname: user.Nickname,
		SubmittedBio:      user.Bio,
		Status:            "pending",
	}
	if err := s.repo.UpdateProfileWithAudit(ctx, user, audit); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) LoginOrCreate(ctx context.Context, authUID, openID string) (*models.User, error) {
	authUID = strings.TrimSpace(authUID)
	openID = strings.TrimSpace(openID)
	if authUID == "" {
		return nil, fmt.Errorf("auth uid is required")
	}

	user, err := s.repo.GetByAuthUID(ctx, authUID)
	if err == nil {
		if openID != "" && user.OpenID != openID {
			user.OpenID = openID
			if err := s.repo.Update(ctx, user); err != nil {
				return nil, err
			}
		}
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	user = &models.User{AuthUID: authUID, OpenID: openID, ProfileStatus: "pending"}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}
