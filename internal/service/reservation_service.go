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

// ReservationSpaceSlotItem is a single bookable court within a time group.
// All fields needed to call ReserveV2 are embedded so the frontend only needs
// to pass them back verbatim — no re-query of dayInfo on submit.
type ReservationSpaceSlotItem struct {
	SlotKey       string // "{spaceId}|{timeId}"
	VenueSiteID   int64
	SpaceID       int64
	SpaceName     string
	Available     bool
	Token         string // top-level token from dayInfo response
	WeekStartDate string // top-level weekStartDate from dayInfo response
}

// ReservationSlotGroupItem groups all courts that share the same time window.
type ReservationSlotGroupItem struct {
	ReservationDate string
	TimeID          int64
	StartTime       string // HH:mm
	EndTime         string // HH:mm
	DisplayLabel    string // "HH:mm-HH:mm"
	Spaces          []ReservationSpaceSlotItem
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
	VenueSiteID     *uint
	SpaceID         *uint
	SpaceName       *string
	// Slot context from ListSlots — when all four are set, TYYS re-query is skipped.
	TimeID        *string
	Token         *string
	WeekStartDate *string
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
	VenueSiteID       *uint
	SpaceID           *uint
	SpaceName         *string
	TimeID            *string
	Token             *string
	WeekStartDate     *string
}

type ReservationService struct {
	roomRepo        *repository.RoomRepository
	reservationRepo *repository.ReservationRepository
	tyys            *zjulogin.TYYS
	captchaSolver   zjulogin.TYYSCaptchaSolver
}

func NewReservationService(roomRepo *repository.RoomRepository, reservationRepo *repository.ReservationRepository, tyys *zjulogin.TYYS, captchaSolver zjulogin.TYYSCaptchaSolver) *ReservationService {
	return &ReservationService{roomRepo: roomRepo, reservationRepo: reservationRepo, tyys: tyys, captchaSolver: captchaSolver}
}

func (s *ReservationService) ListVenues(ctx context.Context, sportType, campus *string) ([]ReservationVenueItem, error) {
	resp, err := s.tyys.VenueInfo(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("get venue info: %w", err)
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
	return result, nil
}

func (s *ReservationService) ListSlots(ctx context.Context, sportType, campusName, venueName, reservationDate string) ([]ReservationSlotGroupItem, error) {
	// Step 1: resolve venueId and venueSiteId from TYYS venue catalogue.
	venueResp, err := s.tyys.VenueInfo(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("get venue info: %w", err)
	}

	var venueID, venueSiteID string
	walkVenues(venueResp.Data, func(obj map[string]any) {
		if venueID != "" {
			return
		}
		if !textMatches(trimString(obj["sportName"]), sportType) {
			return
		}
		if !textMatches(trimString(obj["campusName"]), campusName) {
			return
		}
		if !textMatches(trimString(obj["venueName"]), venueName) {
			return
		}
		venueID = trimString(obj["venueId"])
		venueSiteID = trimString(obj["id"])
	})

	if venueID == "" || venueSiteID == "" {
		return nil, fmt.Errorf("venue not found for sport=%s campus=%s venue=%s", sportType, campusName, venueName)
	}

	venueSiteIDInt, _ := strconv.ParseInt(venueSiteID, 10, 64)

	// Step 2: fetch day info for the requested date.
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

	// token and weekStartDate are top-level fields in the dayInfo response,
	// shared by all slots. Extract once and embed into every space entry.
	token, weekStartDate := extractDayInfoMeta(dayResp.Data)
	if weekStartDate == "" {
		weekStartDate = reservationDate
	}

	// Step 3: walk slots and aggregate by timeId.
	// TYYS stores slots as: space{ id, spaceName, "<timeId>": { startDate, endDate, … } }
	// walkSlotsWithContext passes both the slot child and its parent space object.
	var groups []ReservationSlotGroupItem
	groupIdx := map[string]int{} // timeID string → index in groups

	walkSlotsWithContext(dayResp.Data, func(timeID string, slot, space map[string]any) {
		startFull := trimString(slot["startDate"]) // "YYYY-MM-DD HH:mm"
		endFull := trimString(slot["endDate"])
		startHHmm := extractHHmm(startFull)
		endHHmm := extractHHmm(endFull)

		timeIDInt, _ := strconv.ParseInt(timeID, 10, 64)

		spaceIDStr := trimString(space["id"])
		spaceIDInt, _ := strconv.ParseInt(spaceIDStr, 10, 64)

		sp := ReservationSpaceSlotItem{
			SlotKey:       spaceIDStr + "|" + timeID,
			VenueSiteID:   venueSiteIDInt,
			SpaceID:       spaceIDInt,
			SpaceName:     trimString(space["spaceName"]),
			Available:     isSlotAvailable(slot),
			Token:         token,
			WeekStartDate: weekStartDate,
		}

		if idx, exists := groupIdx[timeID]; exists {
			groups[idx].Spaces = append(groups[idx].Spaces, sp)
		} else {
			groupIdx[timeID] = len(groups)
			groups = append(groups, ReservationSlotGroupItem{
				ReservationDate: reservationDate,
				TimeID:          timeIDInt,
				StartTime:       startHHmm,
				EndTime:         endHHmm,
				DisplayLabel:    startHHmm + "-" + endHHmm,
				Spaces:          []ReservationSpaceSlotItem{sp},
			})
		}
	})

	return groups, nil
}

// Preview validates a proposed reservation against room state and TYYS availability
// without writing anything to the database.
func (s *ReservationService) Preview(ctx context.Context, input ReservationPreviewInput) (*ReservationPreviewOutput, error) {
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
		return nil, fmt.Errorf("sport %s requires at least %d members before reservation (current: %d)",
			input.SportType, sportCfg.MinMemberCount, memberCount)
	}

	venueSiteID, err := s.resolveAndVerifySlot(ctx, input)
	if err != nil {
		return nil, err
	}

	venueSiteIDUint := uint(venueSiteID)
	return &ReservationPreviewOutput{
		RoomID:            input.RoomID,
		Provider:          "tyys",
		ReservationStatus: "pending",
		SportType:         input.SportType,
		CampusName:        input.CampusName,
		VenueName:         input.VenueName,
		ReservationDate:   input.ReservationDate,
		StartTime:         input.StartTime,
		EndTime:           input.EndTime,
		BuddyCode:         input.BuddyCode,
		VenueSiteID:       &venueSiteIDUint,
		SpaceID:           input.SpaceID,
		SpaceName:         input.SpaceName,
		TimeID:            input.TimeID,
		Token:             input.Token,
		WeekStartDate:     input.WeekStartDate,
	}, nil
}

// Submit validates the reservation, writes a RoomReservation record, and determines
// its initial status based on the TYYS opening-time rule.
func (s *ReservationService) Submit(ctx context.Context, input ReservationPreviewInput) (*models.RoomReservation, error) {
	// --- 1. Validate room state ---
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
		return nil, fmt.Errorf("sport %s requires at least %d members before reservation (current: %d)",
			input.SportType, sportCfg.MinMemberCount, memberCount)
	}

	// --- 2. Idempotency check ---
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

	// --- 3. Verify venue / slot availability ---
	venueSiteID, err := s.resolveAndVerifySlot(ctx, input)
	if err != nil {
		return nil, err
	}

	// --- 4. TYYS opening-time rule ---
	openAt, err := tyysOpenTime(input.ReservationDate)
	if err != nil {
		return nil, fmt.Errorf("calculate tyys open time: %w", err)
	}
	status := "scheduled"
	if !time.Now().Before(openAt) {
		status = "submitting"
	}

	// --- 5. Persist ---
	venueSiteIDUint := uint(venueSiteID)
	buddyCode := ""
	if input.BuddyCode != nil {
		buddyCode = *input.BuddyCode
	}
	spaceName := ""
	if input.SpaceName != nil {
		spaceName = *input.SpaceName
	}
	timeID := ""
	if input.TimeID != nil {
		timeID = *input.TimeID
	}
	token := ""
	if input.Token != nil {
		token = *input.Token
	}
	weekStartDate := ""
	if input.WeekStartDate != nil {
		weekStartDate = *input.WeekStartDate
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
		VenueSiteID:       &venueSiteIDUint,
		SpaceID:           input.SpaceID,
		SpaceName:         spaceName,
		TimeID:            timeID,
		Token:             token,
		WeekStartDate:     weekStartDate,
		BuddyCode:         buddyCode,
		ReservationStatus: status,
		ScheduleStatus:    "waiting",
		ReserveOpenAt:     &openAt,
	}
	if err := s.reservationRepo.Create(ctx, reservation); err != nil {
		return nil, fmt.Errorf("create reservation: %w", err)
	}

	// If the TYYS window is already open, trigger in the background.
	// The reservation is returned with status "submitting"; the goroutine
	// updates it to "success" or "failed" once TYYS responds.
	if status == "submitting" {
		reservationID := reservation.ID
		go func() {
			ctx := context.Background()
			if _, err := s.TriggerReservation(ctx, reservationID); err != nil {
				_ = s.logAttempt(ctx, reservation.RoomID, reservationID, "async_trigger", false, err.Error())
			}
		}()
	}

	// --- 6. Mirror status onto room ---
	room.ReservationStatus = status
	if updateErr := s.roomRepo.Update(ctx, room); updateErr != nil {
		_ = s.logAttempt(ctx, input.RoomID, reservation.ID, "update_room_status", false, updateErr.Error())
	}

	_ = s.logAttempt(ctx, input.RoomID, reservation.ID, "submit_plan", true,
		fmt.Sprintf("reservation created with status=%s openAt=%s", status, openAt.Format(time.RFC3339)))

	return reservation, nil
}

// resolveAndVerifySlot returns the resolved venueSiteID (as int64) and verifies
// slot availability. When the full slot context (VenueSiteID, SpaceID, TimeID, Token)
// is already provided from ListSlots, TYYS re-query is skipped entirely.
func (s *ReservationService) resolveAndVerifySlot(ctx context.Context, input ReservationPreviewInput) (int64, error) {
	if slotContextComplete(input) {
		return int64(*input.VenueSiteID), nil
	}

	// Fallback: look up venue and verify slot by start/end time.
	venueID, venueSiteID, err := s.lookupVenueIDs(ctx, input)
	if err != nil {
		return 0, err
	}
	if err := s.checkSlotAvailable(ctx, venueID, venueSiteID, input); err != nil {
		return 0, err
	}
	id, _ := strconv.ParseInt(venueSiteID, 10, 64)
	return id, nil
}

// slotContextComplete returns true when the caller already has all fields
// needed to call ReserveV2 without querying TYYS again.
func slotContextComplete(input ReservationPreviewInput) bool {
	return input.VenueSiteID != nil && input.SpaceID != nil &&
		input.TimeID != nil && input.Token != nil
}

func (s *ReservationService) lookupVenueIDs(ctx context.Context, input ReservationPreviewInput) (venueID, venueSiteID string, err error) {
	venueResp, err := s.tyys.VenueInfo(ctx, 0)
	if err != nil {
		return "", "", fmt.Errorf("get venue info: %w", err)
	}
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
		return "", "", fmt.Errorf("venue not found for sport=%s campus=%s venue=%s",
			input.SportType, input.CampusName, input.VenueName)
	}
	return venueID, venueSiteID, nil
}

// checkSlotAvailable queries TYYS day info and confirms the start/end time slot
// exists and has at least one free court.
func (s *ReservationService) checkSlotAvailable(ctx context.Context, venueID, venueSiteID string, input ReservationPreviewInput) error {
	params := url.Values{}
	params.Set("venueId", venueID)
	params.Set("venueSiteId", venueSiteID)
	params.Set("siteId", venueSiteID)
	params.Set("date", input.ReservationDate)
	params.Set("reservationDate", input.ReservationDate)
	params.Set("searchDate", input.ReservationDate)

	dayResp, err := s.tyys.ReservationDayInfo(ctx, params)
	if err != nil {
		return fmt.Errorf("get day info: %w", err)
	}

	wantStart := input.ReservationDate + " " + input.StartTime
	wantEnd := input.ReservationDate + " " + input.EndTime
	found, available := false, false

	walkSlotsWithContext(dayResp.Data, func(_ string, slot, _ map[string]any) {
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

// tyysOpenTime returns 09:00 CST exactly 2 calendar days before the reservation date.
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

// TriggerReservation executes the TYYS reservation for an existing record.
// It reads the stored slot context (venue_site_id, space_id, time_id, token,
// week_start_date) and calls ReserveV2 directly — no dayInfo re-query is made.
func (s *ReservationService) TriggerReservation(ctx context.Context, reservationID uint) (*models.RoomReservation, error) {
	reservation, err := s.reservationRepo.GetByID(ctx, reservationID)
	if err != nil {
		return nil, fmt.Errorf("get reservation: %w", err)
	}
	if reservation.ReservationStatus != "submitting" {
		return nil, fmt.Errorf("reservation %d has status %q, expected submitting", reservationID, reservation.ReservationStatus)
	}
	if reservation.VenueSiteID == nil || reservation.SpaceID == nil || reservation.TimeID == "" || reservation.Token == "" {
		return nil, fmt.Errorf("reservation %d is missing required slot context", reservationID)
	}

	// Mark as running before calling TYYS.
	now := time.Now()
	reservation.SubmitAttemptedAt = &now
	reservation.ScheduleStatus = "running"
	if err := s.reservationRepo.Update(ctx, reservation); err != nil {
		return nil, fmt.Errorf("update reservation before trigger: %w", err)
	}

	result, tyysErr := s.tyys.ReserveV2(ctx, zjulogin.TYYSReservationV2Request{
		ReservationDate: reservation.ReservationDate,
		WeekStartDate:   reservation.WeekStartDate,
		Token:           reservation.Token,
		VenueSiteID:     fmt.Sprintf("%d", *reservation.VenueSiteID),
		SpaceID:         fmt.Sprintf("%d", *reservation.SpaceID),
		TimeID:          reservation.TimeID,
		BuddyCode:       reservation.BuddyCode,
		CaptchaSolver:   s.captchaSolver,
	})

	success := tyysErr == nil && result != nil && result.Submit != nil
	if success {
		orderID, tradeNo := extractTYYSSubmitOrderInfo(result.Submit.Data)
		reservation.ExternalOrderID = orderID
		reservation.ExternalTradeNo = tradeNo
		reservation.RawResponse = string(result.Submit.Data)
		reservation.ReservationStatus = "success"
		reservation.ScheduleStatus = "done"
	} else {
		reservation.ReservationStatus = "failed"
		reservation.ScheduleStatus = "error"
		if result != nil && result.Submit != nil {
			reservation.RawResponse = string(result.Submit.Data)
		}
	}

	if err := s.reservationRepo.Update(ctx, reservation); err != nil {
		_ = s.logAttempt(ctx, reservation.RoomID, reservationID, "trigger_update", false, err.Error())
		return nil, fmt.Errorf("update reservation after trigger: %w", err)
	}

	// Mirror status onto room.
	if room, err := s.roomRepo.GetByID(ctx, reservation.RoomID); err == nil {
		room.ReservationStatus = reservation.ReservationStatus
		_ = s.roomRepo.Update(ctx, room)
	}

	msg := ""
	if tyysErr != nil {
		msg = tyysErr.Error()
	}
	_ = s.logAttempt(ctx, reservation.RoomID, reservationID, "trigger_reservation", success, msg)

	if tyysErr != nil {
		return reservation, fmt.Errorf("tyys reserve: %w", tyysErr)
	}
	return reservation, nil
}

// extractTYYSSubmitOrderInfo pulls order identifiers from a TYYS submit response.
func extractTYYSSubmitOrderInfo(data json.RawMessage) (orderID, tradeNo string) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return
	}
	for _, key := range []string{"orderId", "orderSn", "id"} {
		if v := trimString(obj[key]); v != "" {
			orderID = v
			break
		}
	}
	tradeNo = trimString(obj["tradeNo"])
	return
}

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

// ── sport config ─────────────────────────────────────────────────────────────

type sportConfig struct {
	RequiresBuddyCode bool
	MinMemberCount    int
}

// getSportConfig returns booking rules for a sport type.
// Badminton and tennis require a buddy code and at least 2 members.
func getSportConfig(sportType string) sportConfig {
	switch sportType {
	case "羽毛球", "网球":
		return sportConfig{RequiresBuddyCode: true, MinMemberCount: 2}
	default:
		return sportConfig{RequiresBuddyCode: false, MinMemberCount: 1}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

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

func textMatches(got, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return true
	}
	got = strings.TrimSpace(got)
	return got == want || strings.Contains(got, want) || strings.Contains(want, got)
}

// extractHHmm extracts "HH:mm" from a "YYYY-MM-DD HH:mm" string.
func extractHHmm(datetime string) string {
	if i := strings.LastIndex(datetime, " "); i >= 0 {
		return datetime[i+1:]
	}
	return datetime
}

// extractDayInfoMeta reads token and weekStartDate from the top level of a
// TYYS dayInfo response. Both fields are shared across all slots in the response.
func extractDayInfoMeta(data []byte) (token, weekStartDate string) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return
	}
	token = trimString(obj["token"])
	weekStartDate = trimString(obj["weekStartDate"])
	return
}

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

// walkSlotsWithContext walks the TYYS dayInfo response and calls visit for each
// slot object (identified by having a "startDate" field), passing the timeID
// (the map key under which the slot is stored) and the parent space object.
// TYYS structure: space{ id, spaceName, "<timeId>": { startDate, endDate, … } }
func walkSlotsWithContext(data []byte, visit func(timeID string, slot, space map[string]any)) {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for k, child := range typed {
				if obj, ok := child.(map[string]any); ok {
					if _, hasStart := obj["startDate"]; hasStart {
						visit(k, obj, typed) // typed is the parent space object
					} else {
						walk(obj)
					}
				} else {
					walk(child)
				}
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(payload)
}
