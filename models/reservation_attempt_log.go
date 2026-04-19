package models

import "time"

type ReservationAttemptLog struct {
	ID               uint   `gorm:"primaryKey"`
	RoomID           *uint  `gorm:"index"`
	ReservationID    *uint  `gorm:"index"`
	Stage            string `gorm:"size:32;not null;index"`
	Success          bool   `gorm:"not null;default:false"`
	Message          string `gorm:"size:255"`
	RequestSnapshot  string `gorm:"type:text"`
	ResponseSnapshot string `gorm:"type:text"`
	CreatedAt        time.Time
	Room             *Room            `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;foreignKey:RoomID;references:ID"`
	Reservation      *RoomReservation `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;foreignKey:ReservationID;references:ID"`
}
