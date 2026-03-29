package models

import "time"

type RoomMember struct {
	ID        uint       `gorm:"primaryKey"`
	RoomID    uint       `gorm:"not null;uniqueIndex:uk_room_member;index;index:idx_room_member_room_status,priority:1"`
	UserID    uint       `gorm:"not null;uniqueIndex:uk_room_member;index"`
	Role      string     `gorm:"size:32;not null"`
	Status    string     `gorm:"size:32;not null;index:idx_room_member_room_status,priority:2"`
	JoinedAt  *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	Room      Room       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:RoomID;references:ID"`
	User      User       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;foreignKey:UserID;references:ID"`
}
