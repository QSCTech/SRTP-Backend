
package repository

import (
	"context"

	"github.com/QSCTech/SRTP-Backend/models"
	"gorm.io/gorm"
)

type AuditRepository struct {
	db *gorm.DB		
}

func NewAuditRepository(db *gorm.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Create(ctx context.Context, audit *models.UserProfileAudit) error {
	return r.db.WithContext(ctx).Create(audit).Error
}

func (r *AuditRepository) GetLatestByUserID(ctx context.Context, userID uint) (*models.UserProfileAudit, error) {
	var audit models.UserProfileAudit
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		First(&audit).Error
	if err != nil {
		return nil, err
	}
	return &audit, nil
}

func (r *AuditRepository) UpdateUserProfile(ctx context.Context, userID uint, nickname, bio string) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"nickname":       nickname,
			"bio":            bio,
			"profile_status": "approved", 
		}).Error
}