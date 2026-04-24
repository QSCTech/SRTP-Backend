package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/internal/zjulogin"
	"github.com/QSCTech/SRTP-Backend/models"
	"gorm.io/gorm"
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

	// Step 3: Parse slots from day info response.
	// Use walkSlotsWithTimeID so the parent map key (which is the TYYS timeId)
	// is captured as SlotKey — slot objects themselves do not contain a timeId field.
	var slots []ReservationSlotItem
	walkSlotsWithTimeID(dayResp.Data, func(timeID string, slot map[string]any) {
		item := ReservationSlotItem{
			SlotKey:   timeID,
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

// walkSlotsWithTimeID is like walkSlots but also captures the parent map key,
// which is the TYYS timeId. TYYS stores slot objects as values keyed by timeId
// inside a space object, so the timeId is only visible from the parent's perspective.
func walkSlotsWithTimeID(data []byte, visit func(timeID string, slot map[string]any)) {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}
	walkJSONObjectsWithKey(payload, func(key string, obj map[string]any) {
		if _, hasStart := obj["startDate"]; hasStart {
			visit(key, obj)
		}
	})
}

// walkJSONObjectsWithKey recursively walks a parsed JSON structure and calls
// visit for each map entry whose value is itself a map, passing the entry key.
// This lets callers see the key under which each object is stored.
func walkJSONObjectsWithKey(value any, visit func(key string, obj map[string]any)) {
	switch typed := value.(type) {
	case map[string]any:
		for k, child := range typed {
			if obj, ok := child.(map[string]any); ok {
				visit(k, obj)
				walkJSONObjectsWithKey(obj, visit)
			} else {
				walkJSONObjectsWithKey(child, visit)
			}
		}
	case []any:
		for _, child := range typed {
			walkJSONObjectsWithKey(child, visit)
		}
	}
}

// Preview validates a proposed reservation against the room state and TYYS availability
// without writing anything to the database. The caller uses the returned output to confirm
// details before calling Submit.
func (s *ReservationService) Preview(ctx context.Context, input ReservationPreviewInput) (*ReservationPreviewOutput, error) {
	room, err := s.roomRepo.GetByID(ctx, input.RoomID)
	if err != nil {
		return nil, fmt.Errorf("room not found: %w", err)
	}
	if !room.NeedReservation {
		return nil, fmt.Errorf("room does not require reservation")
	}
	// Only active rooms (recruiting / full) may enter the reservation flow.
	if room.Status == "cancelled" || room.Status == "finished" {
		return nil, fmt.Errorf("room is not active (status=%s)", room.Status)
	}
	sportCfg := getSportConfig(input.SportType)

	// TYYS requires a buddy/partner code when booking pair courts (羽毛球, 网球).
	// Fail early here so the user sees a clear error before any network call.
	if sportCfg.RequiresBuddyCode && (input.BuddyCode == nil || strings.TrimSpace(*input.BuddyCode) == "") {
		return nil, fmt.Errorf("sport %s requires a buddy code", input.SportType)
	}

	// Pair sports need a minimum number of joined members before reservation makes sense.
	memberCount, err := s.roomRepo.CountActiveMembers(ctx, input.RoomID)
	if err != nil {
		return nil, fmt.Errorf("count active members: %w", err)
	}
	if int(memberCount) < sportCfg.MinMemberCount {
		return nil, fmt.Errorf("sport %s requires at least %d members before reservation (current: %d)", input.SportType, sportCfg.MinMemberCount, memberCount)
	}

	venueID, venueSiteID, err := s.resolveVenueIDs(ctx, input)
	if err != nil {
		return nil, err
	}

	if err := s.checkSlotAvailable(ctx, venueID, venueSiteID, input); err != nil {
		return nil, err
	}

	// TYYS returns IDs as strings; convert back to uint so the response carries
	// the resolved IDs that Submit can reuse without a second venue lookup.
	venueIDUint, _ := strconv.ParseUint(venueID, 10, 64)
	venueSiteIDUint, _ := strconv.ParseUint(venueSiteID, 10, 64)
	venueIDResult := uint(venueIDUint)
	venueSiteIDResult := uint(venueSiteIDUint)

	return &ReservationPreviewOutput{
		RoomID:            input.RoomID,
		Provider:          "tyys",
		ReservationStatus: "pending", // preview does not submit; status stays pending until Submit succeeds
		SportType:         input.SportType,
		CampusName:        input.CampusName,
		VenueName:         input.VenueName,
		ReservationDate:   input.ReservationDate,
		StartTime:         input.StartTime,
		EndTime:           input.EndTime,
		BuddyCode:         input.BuddyCode,
		VenueID:           &venueIDResult,
		VenueSiteID:       &venueSiteIDResult,
		SpaceID:           input.SpaceID,
		SpaceName:         input.SpaceName,
	}, nil
}


// resolveVenueIDs returns the TYYS venueId and venueSiteId for the requested venue.
// If the caller already obtained these IDs (e.g. from ListSlots), they are returned
// directly to avoid an extra round-trip to the TYYS venue-info API.
func (s *ReservationService) resolveVenueIDs(ctx context.Context, input ReservationPreviewInput) (venueID, venueSiteID string, err error) {
	if input.VenueID != nil && input.VenueSiteID != nil {
		return strconv.FormatUint(uint64(*input.VenueID), 10), strconv.FormatUint(uint64(*input.VenueSiteID), 10), nil
	}

	venueResp, err := s.tyys.VenueInfo(ctx, 0)
	if err != nil {
		return "", "", fmt.Errorf("get venue info: %w", err)
	}

	// walkVenues has no early-exit mechanism; guard with venueID != "" to stop processing
	// after the first match and avoid overwriting with a later duplicate entry.
	walkVenues(venueResp.Data, func(obj map[string]any) {
		if venueID != "" {
			return
		}
		if !textMatches(trimString(obj["sportName"]), input.SportType) {
			return
		}
		if !textMatches(trimString(obj["campusName"]), input.CampusName) {
			return
		}
		if !textMatches(trimString(obj["venueName"]), input.VenueName) {
			return
		}
		venueID = trimString(obj["venueId"])
		venueSiteID = trimString(obj["id"])
	})

	if venueID == "" || venueSiteID == "" {
		return "", "", fmt.Errorf("venue not found for sport=%s campus=%s venue=%s", input.SportType, input.CampusName, input.VenueName)
	}
	return venueID, venueSiteID, nil
}

// checkSlotAvailable queries TYYS day info and confirms the specific start/end time slot
// exists and is free. The TYYS API stores slot times as "YYYY-MM-DD HH:mm", so we
// concatenate reservationDate + " " + time before comparing against startDate/endDate.
func (s *ReservationService) checkSlotAvailable(ctx context.Context, venueID, venueSiteID string, input ReservationPreviewInput) error {
	params := url.Values{}
	params.Set("venueId", venueID)
	params.Set("venueSiteId", venueSiteID)
	params.Set("siteId", venueSiteID) // TYYS day-info API accepts both siteId and venueSiteId; send both for compatibility
	params.Set("date", input.ReservationDate)
	params.Set("reservationDate", input.ReservationDate)
	params.Set("searchDate", input.ReservationDate)

	dayResp, err := s.tyys.ReservationDayInfo(ctx, params)
	if err != nil {
		return fmt.Errorf("get day info: %w", err)
	}

	// TYYS slot timestamps are formatted as "YYYY-MM-DD HH:mm".
	wantStart := input.ReservationDate + " " + input.StartTime
	wantEnd := input.ReservationDate + " " + input.EndTime
	found := false
	available := false
	walkSlots(dayResp.Data, func(slot map[string]any) {
		// A venue has multiple courts; the same time window appears once per court.
		// Stop early once we find an available court — we don't need all of them.
		if available {
			return
		}
		if trimString(slot["startDate"]) == wantStart && trimString(slot["endDate"]) == wantEnd {
			found = true
			if isSlotAvailable(slot) {
				available = true
			}
		}
	})

	if !found {
		return fmt.Errorf("slot not found for %s %s-%s", input.ReservationDate, input.StartTime, input.EndTime)
	}
	if !available {
		return fmt.Errorf("slot %s %s-%s is not available", input.ReservationDate, input.StartTime, input.EndTime)
	}
	return nil
}

// Submit validates the reservation request, writes a RoomReservation record, and
// determines its initial status based on the TYYS opening-time rule:
//   - Before the window: status="scheduled", ReserveOpenAt records when to trigger
//   - After the window:  status="submitting", ready for the scheduler to pick up
//
// The actual TYYS call (which requires captcha solving) is NOT made here;
// it is delegated to POST /internal/tasks/reservation-trigger so the HTTP
// response stays fast and captcha complexity lives in the scheduler.
func (s *ReservationService) Submit(ctx context.Context, input ReservationPreviewInput) (*models.RoomReservation, error) {
	// --- 1. Validate room state (same gates as Preview) ---
	room, err := s.roomRepo.GetByID(ctx, input.RoomID)
	if err != nil {
		return nil, fmt.Errorf("room not found: %w", err)
	}
	if !room.NeedReservation {
		return nil, fmt.Errorf("room does not require reservation")
	}
	if room.Status == "cancelled" || room.Status == "finished" {
		return nil, fmt.Errorf("room is not active (status=%s)", room.Status)
	}
	sportCfg := getSportConfig(input.SportType)
	if sportCfg.RequiresBuddyCode && (input.BuddyCode == nil || strings.TrimSpace(*input.BuddyCode) == "") {
		return nil, fmt.Errorf("sport %s requires a buddy code", input.SportType)
	}

	memberCount, err := s.roomRepo.CountActiveMembers(ctx, input.RoomID)
	if err != nil {
		return nil, fmt.Errorf("count active members: %w", err)
	}
	if int(memberCount) < sportCfg.MinMemberCount {
		return nil, fmt.Errorf("sport %s requires at least %d members before reservation (current: %d)", input.SportType, sportCfg.MinMemberCount, memberCount)
	}

	// --- 2. Idempotency: reject if an active reservation already exists ---
	existing, err := s.reservationRepo.GetLatestByRoomID(ctx, input.RoomID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("check existing reservation: %w", err)
	}
	if existing != nil {
		switch existing.ReservationStatus {
		case "scheduled", "submitting", "success":
			return nil, fmt.Errorf("room already has an active reservation (status=%s)", existing.ReservationStatus)
		}
	}

	// --- 3. Verify venue and slot availability on TYYS ---
	venueID, venueSiteID, err := s.resolveVenueIDs(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.checkSlotAvailable(ctx, venueID, venueSiteID, input); err != nil {
		return nil, err
	}

	// --- 4. Apply TYYS opening-time rule ---
	// TYYS only accepts reservations starting at 09:00 CST exactly 2 days before
	// the reservation date. Before that moment we create a "scheduled" plan;
	// the scheduler flips it to "submitting" once the window opens.
	openAt, err := tyysOpenTime(input.ReservationDate)
	if err != nil {
		return nil, fmt.Errorf("calculate tyys open time: %w", err)
	}

	status := "scheduled"
	if !time.Now().Before(openAt) {
		// Window is already open — mark for immediate scheduler pickup.
		status = "submitting"
	}

	// --- 5. Persist the reservation record ---
	venueIDUint, _ := strconv.ParseUint(venueID, 10, 64)
	venueSiteIDUint, _ := strconv.ParseUint(venueSiteID, 10, 64)
	venueIDResult := uint(venueIDUint)
	venueSiteIDResult := uint(venueSiteIDUint)

	buddyCode := ""
	if input.BuddyCode != nil {
		buddyCode = *input.BuddyCode
	}
	spaceName := ""
	if input.SpaceName != nil {
		spaceName = *input.SpaceName
	}

	reservation := &models.RoomReservation{
		RoomID:            input.RoomID,
		Provider:          "tyys",
		SportType:         input.SportType,
		CampusName:        input.CampusName,
		VenueName:         input.VenueName,
		ReservationDate:   input.ReservationDate,
		StartTime:         input.StartTime,
		EndTime:           input.EndTime,
		VenueID:           &venueIDResult,
		VenueSiteID:       &venueSiteIDResult,
		SpaceID:           input.SpaceID,
		SpaceName:         spaceName,
		BuddyCode:         buddyCode,
		ReservationStatus: status,
		ScheduleStatus:    "waiting",
		ReserveOpenAt:     &openAt,
	}
	if err := s.reservationRepo.Create(ctx, reservation); err != nil {
		return nil, fmt.Errorf("create reservation: %w", err)
	}

	// --- 6. Mirror status onto the room so the room card reflects it ---
	room.ReservationStatus = status
	if updateErr := s.roomRepo.Update(ctx, room); updateErr != nil {
		// Non-fatal: reservation is already saved; log and continue.
		_ = s.logAttempt(ctx, input.RoomID, reservation.ID, "update_room_status", false, updateErr.Error())
	}

	// --- 7. Record a successful plan-creation attempt ---
	_ = s.logAttempt(ctx, input.RoomID, reservation.ID, "submit_plan", true,
		fmt.Sprintf("reservation created with status=%s openAt=%s", status, openAt.Format(time.RFC3339)))

	return reservation, nil
}

// tyysOpenTime returns the moment at which TYYS accepts bookings for a given
// reservation date: 09:00 CST exactly 2 calendar days before that date.
func tyysOpenTime(reservationDate string) (time.Time, error) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Time{}, fmt.Errorf("load Asia/Shanghai location: %w", err)
	}
	date, err := time.ParseInLocation("2006-01-02", reservationDate, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse reservation date %q: %w", reservationDate, err)
	}
	open := date.AddDate(0, 0, -2)
	return time.Date(open.Year(), open.Month(), open.Day(), 9, 0, 0, 0, loc), nil
}

// logAttempt writes a ReservationAttemptLog record; errors are ignored by callers
// because logging failures must not abort the main flow.
func (s *ReservationService) logAttempt(ctx context.Context, roomID, reservationID uint, stage string, success bool, message string) error {
	entry := &models.ReservationAttemptLog{
		RoomID:        &roomID,
		ReservationID: &reservationID,
		Stage:         stage,
		Success:       success,
		Message:       message,
	}
	return s.reservationRepo.CreateAttemptLog(ctx, entry)
}
