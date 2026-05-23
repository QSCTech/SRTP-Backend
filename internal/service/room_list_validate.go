package service

import (
	"fmt"
	"strings"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
)

var allowedTimeRanges = map[string]struct{}{
	"morning":   {},
	"afternoon": {},
	"evening":   {},
}

func normalizeListRoomsInput(input *ListRoomsInput) (repository.RoomSort, error) {
	if input.Keyword != nil {
		trimmed := strings.TrimSpace(*input.Keyword)
		if trimmed == "" {
			input.Keyword = nil
		} else {
			input.Keyword = &trimmed
		}
	}

	if input.TimeRange != nil {
		value := strings.TrimSpace(*input.TimeRange)
		if value == "" {
			input.TimeRange = nil
		} else if _, ok := allowedTimeRanges[value]; !ok {
			return "", fmt.Errorf("invalid time_range: %s", value)
		} else {
			input.TimeRange = &value
		}
	}

	sort, err := repository.ParseRoomSort(strings.TrimSpace(input.Sort))
	if err != nil {
		return "", err
	}

	if input.Page < 1 {
		input.Page = 1
	}
	if input.PageSize < 1 {
		input.PageSize = 20
	}
	if input.PageSize > 100 {
		input.PageSize = 100
	}

	return sort, nil
}
