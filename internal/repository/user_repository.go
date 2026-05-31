package repository

import (
	"context"

	"github.com/QSCTech/SRTP-Backend/models"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepository) GetByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByPublicID(ctx context.Context, publicID string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("public_id = ?", publicID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByAuthUID(ctx context.Context, authUID string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("auth_uid = ?", authUID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetFirst(ctx context.Context) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Order("id ASC").First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *UserRepository) CreateProfileAudit(ctx context.Context, audit *models.UserProfileAudit) error {
	return r.db.WithContext(ctx).Create(audit).Error
}

func (r *UserRepository) UpdateProfileWithAudit(ctx context.Context, user *models.User, audit *models.UserProfileAudit) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(user).Error; err != nil {
			return err
		}
		return tx.Create(audit).Error
	})
}
