package models

import "time"

type Room struct {
	ID                uint       `gorm:"primaryKey"`
	OwnerID           uint       `gorm:"not null;index"`
	Name              string     `gorm:"size:32;not null"`
	SportCode         string     `gorm:"size:32;not null;index"`
	Visibility        string     `gorm:"size:32;not null;default:'public'"`
	JoinPolicy        string     `gorm:"size:32;not null;default:'direct_join'"`
	Status            string     `gorm:"size:32;not null;default:'open';index"`
	LocationText      string     `gorm:"size:128;not null"`
	StartTime         time.Time  `gorm:"not null;index"`
	EndTime           *time.Time
	GenderRequirement string     `gorm:"size:32"`
	MinMembers        *int
	MaxMembers        *int
	OrganizationText  string     `gorm:"size:64"`
	SkillLevel        string     `gorm:"size:32"`
	Atmosphere        string     `gorm:"size:32"`
	Description       string     `gorm:"size:500"`
	InviteCode        string     `gorm:"uniqueIndex;size:16"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Owner             User       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;foreignKey:OwnerID;references:ID"`
}
