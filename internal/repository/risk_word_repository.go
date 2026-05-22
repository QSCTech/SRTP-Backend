package repository

import (
	"context"

	"github.com/QSCTech/SRTP-Backend/models" 
	"gorm.io/gorm"                      
)

type RiskWordRepository struct {
	db *gorm.DB 
}

func NewRiskWordRepository(db *gorm.DB) *RiskWordRepository {
	return &RiskWordRepository{db: db}
}

func (r *RiskWordRepository) GetAllEnabled(ctx context.Context) ([]models.RiskWord, error) {
	var words []models.RiskWord 

	err := r.db.WithContext(ctx).
					Where("enabled = ?", true).
					Order("id ASC").    
					Find(&words).Error        

	if err != nil {
		return nil, err
	}
	return words, nil
}
