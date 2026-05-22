package models 

import "time" 

type RiskWord struct {
	ID      uint   `gorm:"primaryKey"`       
	Pattern string `gorm:"size:255;not null"` 
	Category string `gorm:"size:32"` 
	Enabled bool `gorm:"not null;default:true"` 
	CreatedAt time.Time
	UpdatedAt time.Time
}