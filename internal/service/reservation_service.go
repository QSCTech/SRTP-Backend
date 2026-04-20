package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/internal/zjulogin"
	"github.com/QSCTech/SRTP-Backend/models"
)

type ReservationVenueItem struct {
	SportType  string
	CampusName string
	VenueName  string
}

type ReservationSlotItem struct {
	SlotKey   string
	StartTime string
	EndTime   string
	Available bool
	SpaceName *string
}

type ReservationPreviewInput struct {
	RoomID          uint
	SportType       string
	CampusName      string
	VenueName       string
	ReservationDate string
	StartTime       string
	EndTime         string
	BuddyCode       *string
	VenueID         *uint
	VenueSiteID     *uint
	SpaceID         *uint
	SpaceName       *string
}

type ReservationPreviewOutput struct {
	RoomID            uint
	Provider          string
	ReservationStatus string
	SportType         string
	CampusName        string
	VenueName         string
	ReservationDate   string
	StartTime         string
	EndTime           string
	BuddyCode         *string
	VenueID           *uint
	VenueSiteID       *uint
	SpaceID           *uint
	SpaceName         *string
}

type ReservationService struct {
	roomRepo        *repository.RoomRepository
	reservationRepo *repository.ReservationRepository
	tyys            *zjulogin.TYYS
}

func NewReservationService(roomRepo *repository.RoomRepository, reservationRepo *repository.ReservationRepository, tyys *zjulogin.TYYS) *ReservationService {
	return &ReservationService{roomRepo: roomRepo, reservationRepo: reservationRepo, tyys: tyys}
}

func (s *ReservationService) ListVenues(ctx context.Context, sportType, campus *string) []ReservationVenueItem {
	resp, err := s.tyys.VenueInfo(ctx, 0)
	if err != nil || resp == nil {
		return nil
	}

	var result []ReservationVenueItem
	walkVenues(resp.Data, func(obj map[string]any) {
		sport := trimString(obj["sportName"])
		camp := trimString(obj["campusName"])
		venue := trimString(obj["venueName"])

		if sport == "" || camp == "" || venue == "" {
			return
		}
		if sportType != nil && *sportType != "" && sport != *sportType {
			return
		}
		if campus != nil && *campus != "" && camp != *campus {
			return
		}
		result = append(result, ReservationVenueItem{
			SportType:  sport,
			CampusName: camp,
			VenueName:  venue,
		})
	})
	return result
}

// trimString converts an any value to string, returning empty string if not a string.
func trimString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatFloat(val, 'f', 0, 64)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	}
	return ""
}

// walkVenues parses TYYS venue data and visits each venue object that has sportName field.
// It recursively walks through the JSON data structure to find all venue objects.
func walkVenues(data []byte, visit func(map[string]any)) {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}
	walkJSONObjects(payload, func(obj map[string]any) {
		if _, ok := obj["sportName"]; ok {
			visit(obj)
		}
	})
}

// walkJSONObjects recursively walks a parsed JSON structure and calls visit for each object.
// It handles both maps and arrays, drilling down into nested structures.
func walkJSONObjects(value any, visit func(map[string]any)) {
	switch typed := value.(type) {
	case map[string]any:
		visit(typed)
		for _, child := range typed {
			walkJSONObjects(child, visit)
		}
	case []any:
		for _, child := range typed {
			walkJSONObjects(child, visit)
		}
	}
}

// textMatches is a flexible string matcher for reservation fields.
// It returns true if want is empty or if got contains want or equals want.
func textMatches(got, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return true
	}
	got = strings.TrimSpace(got)
	return got == want || strings.Contains(got, want) || strings.Contains(want, got)
}

func (s *ReservationService) ListSlots(ctx context.Context, sportType, campusName, venueName, reservationDate string) ([]ReservationSlotItem, error) {
	// Step 1: Get venue info to find venueId and venueSiteId
	venueResp, err := s.tyys.VenueInfo(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("get venue info: %w", err)
	}

	// Find matching venue and extract IDs
	var venueID, venueSiteID string
	walkVenues(venueResp.Data, func(obj map[string]any) {
		if venueID != "" {
			return // already found
		}
		sportGot := trimString(obj["sportName"])
		campusGot := trimString(obj["campusName"])
		venueGot := trimString(obj["venueName"])
		if !textMatches(sportGot, sportType) {
			return
		}
		if !textMatches(campusGot, campusName) {
			return
		}
		if !textMatches(venueGot, venueName) {
			return
		}
		venueID = trimString(obj["venueId"])
		venueSiteID = trimString(obj["id"])
	})

	if venueID == "" || venueSiteID == "" {
		return nil, fmt.Errorf("venue not found for sport=%s campus=%s venue=%s", sportType, campusName, venueName)
	}

	// Step 2: Get day info (available slots)
	params := url.Values{}
	params.Set("venueId", venueID)
	params.Set("venueSiteId", venueSiteID)
	params.Set("siteId", venueSiteID)
	params.Set("date", reservationDate)
	params.Set("reservationDate", reservationDate)
	params.Set("searchDate", reservationDate)

	dayResp, err := s.tyys.ReservationDayInfo(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("get day info: %w", err)
	}

	// Step 3: Parse slots from day info response
	var slots []ReservationSlotItem
	walkSlots(dayResp.Data, func(slot map[string]any) {
		item := ReservationSlotItem{
			SlotKey:   trimString(slot["timeId"]),
			StartTime: trimString(slot["startDate"]),
			EndTime:   trimString(slot["endDate"]),
			Available: isSlotAvailable(slot),
		}
		if name := trimString(slot["spaceName"]); name != "" {
			item.SpaceName = &name
		}
		slots = append(slots, item)
	})

	return slots, nil
}

// isSlotAvailable checks if a slot is available for booking.
func isSlotAvailable(slot map[string]any) bool {
	if status, ok := slot["reservationStatus"].(float64); ok && status != 1 {
		return false
	}
	if count, ok := slot["alreadyNum"].(float64); ok && count > 0 {
		return false
	}
	if tradeNo := trimString(slot["tradeNo"]); tradeNo != "" && tradeNo != "null" {
		return false
	}
	return true
}

// walkSlots walks through parsed JSON and visits each slot object.
func walkSlots(data []byte, visit func(map[string]any)) {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}
	walkJSONObjects(payload, func(obj map[string]any) {
		// A slot object has startDate and endDate fields
		if _, hasStart := obj["startDate"]; hasStart {
			visit(obj)
		}
	})
}

func (s *ReservationService) Preview(ctx context.Context, input ReservationPreviewInput) (*ReservationPreviewOutput, error) {
	return nil, fmt.Errorf("reservation service Preview not implemented")
}

func (s *ReservationService) Submit(ctx context.Context, input ReservationPreviewInput) (*models.RoomReservation, error) {
	return nil, fmt.Errorf("reservation service Submit not implemented")
}
