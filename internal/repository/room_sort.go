package repository

import (
	"fmt"

	"gorm.io/gorm"
)

type RoomSort string

const (
	RoomSortStartTimeAsc  RoomSort = "start_time_asc"
	RoomSortStartTimeDesc RoomSort = "start_time_desc"
	RoomSortCreatedAtDesc RoomSort = "created_at_desc"
)

func ParseRoomSort(raw string) (RoomSort, error) {
	switch RoomSort(raw) {
	case "", RoomSortStartTimeAsc, RoomSortStartTimeDesc, RoomSortCreatedAtDesc:
		if raw == "" {
			return RoomSortStartTimeAsc, nil
		}
		return RoomSort(raw), nil
	default:
		return "", fmt.Errorf("invalid sort: %s", raw)
	}
}

func (s RoomSort) Apply(q *gorm.DB) *gorm.DB {
	switch s {
	case RoomSortStartTimeDesc:
		return q.Order("start_time DESC")
	case RoomSortCreatedAtDesc:
		return q.Order("created_at DESC")
	default:
		return q.Order("start_time ASC")
	}
}
