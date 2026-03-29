package models

import "time"

type UserProfileAudit struct {
	ID                uint       `gorm:"primaryKey"`
	UserID            uint       `gorm:"not null;index;index:idx_user_profile_audit_user_created,priority:1"`
	SubmittedNickname string     `gorm:"size:30"`
	SubmittedBio      string     `gorm:"size:255"`
	Status            string     `gorm:"size:32;not null;default:'pending';index"`
	ReviewedBy        *uint      `gorm:"index"`
	ReviewedAt        *time.Time
	RejectReason      string     `gorm:"size:255"`
	CreatedAt         time.Time  `gorm:"index:idx_user_profile_audit_user_created,priority:2,sort:desc"`
	UpdatedAt         time.Time
	User              User       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;foreignKey:UserID;references:ID"`
	Reviewer          *User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;foreignKey:ReviewedBy;references:ID"`
}
