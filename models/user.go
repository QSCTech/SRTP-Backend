package models

import "time"

type User struct {
	ID            uint      `gorm:"primaryKey"`
	AuthUID       string    `gorm:"uniqueIndex;size:64;not null"`
	Nickname      string    `gorm:"size:30"`
	AvatarURL     string    `gorm:"size:255"`
	Gender        string    `gorm:"size:16"`
	Bio           string    `gorm:"size:255"`
	ProfileStatus string    `gorm:"size:32;not null;default:'pending'"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
