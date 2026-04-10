package zjulogin

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type TYYSReservationRequest struct {
	SportName  string
	CampusName string
	VenueName  string

	VenueID     string
	VenueSiteID string
	IsArt       int

	Date          string
	WeekStartDate string
	StartTime     string
	EndTime       string

	CourtNames []string
	MinCourtNo int
	MaxCourtNo int

	BuddyCode       string
	BuddyID         string
	BuddyIDs        string
	IsOfflineTicket string
	Phone           string

	CaptchaSolver TYYSCaptchaSolver

	DayInfoExtra   url.Values
	OrderInfoExtra url.Values
	SubmitExtra    url.Values
	CaptchaParams  url.Values
}

type TYYSReservationResult struct {
	VenueInfo     *TYYSAPIResponse
	DayInfo       *TYYSAPIResponse
	OrderPreview  *TYYSAPIResponse
	CaptchaCheck  *TYYSAPIResponse
	Submit        *TYYSAPIResponse
	SelectedSlot  map[string]any
	OrderForm     url.Values
	CaptchaAnswer url.Values
}

func (s *TYYS) Reserve(ctx context.Context, req TYYSReservationRequest) (*TYYSReservationResult, error) {
	if strings.TrimSpace(req.Date) == "" || strings.TrimSpace(req.StartTime) == "" || strings.TrimSpace(req.EndTime) == "" {
		return nil, fmt.Errorf("date, start time, and end time are required")
	}
	if req.CaptchaSolver == nil {
		return nil, fmt.Errorf("captcha solver is required")
	}

	resolved, venueInfo, err := s.resolveReservationVenue(ctx, req)
	if err != nil {
		return nil, err
	}
	req = resolved

	dayParams := req.dayInfoParams()
	dayInfo, err := s.ReservationDayInfo(ctx, dayParams)
	if err != nil {
		return &TYYSReservationResult{VenueInfo: venueInfo, DayInfo: dayInfo}, fmt.Errorf("reservation day info params=%s: %w", dayParams.Encode(), err)
	}

	selected, err := selectTYYSReservationSlot(dayInfo.Data, req)
	if err != nil {
		return &TYYSReservationResult{VenueInfo: venueInfo, DayInfo: dayInfo}, err
	}

	orderForm := req.orderInfoForm(selected)
	mergeTYYSReservationMetadata(orderForm, dayInfo.Data, req)
	mergeURLValues(orderForm, req.OrderInfoExtra)

	preview, err := s.ReservationOrderInfo(ctx, orderForm)
	if err != nil {
		return &TYYSReservationResult{VenueInfo: venueInfo, DayInfo: dayInfo, SelectedSlot: selected, OrderForm: orderForm, OrderPreview: preview}, err
	}

	mergeTYYSOrderPreviewSubmitFields(orderForm, preview.Data, req)
	if err := s.addReservationBuddyFields(ctx, orderForm, preview.Data, req); err != nil {
		return &TYYSReservationResult{VenueInfo: venueInfo, DayInfo: dayInfo, SelectedSlot: selected, OrderForm: orderForm, OrderPreview: preview}, err
	}

	captchaParams := cloneValues(req.CaptchaParams)
	if captchaParams == nil {
		captchaParams = url.Values{}
	}
	if captchaParams.Get("captchaType") == "" {
		captchaParams.Set("captchaType", "clickWord")
	}
	if captchaParams.Get("clientUid") == "" {
		captchaParams.Set("clientUid", "point-"+reservationRandomUUIDLikeString())
	}
	if captchaParams.Get("ts") == "" {
		captchaParams.Set("ts", fmt.Sprintf("%d", time.Now().UnixMilli()))
	}
	captchaCheck, captchaAnswer, err := s.CaptchaGetSolveAndCheck(ctx, captchaParams, req.CaptchaSolver)
	if err != nil {
		return &TYYSReservationResult{VenueInfo: venueInfo, DayInfo: dayInfo, SelectedSlot: selected, OrderForm: orderForm, OrderPreview: preview, CaptchaCheck: captchaCheck, CaptchaAnswer: captchaAnswer}, err
	}
	if err := requireTYYSCaptchaCheckOK(captchaCheck); err != nil {
		return &TYYSReservationResult{VenueInfo: venueInfo, DayInfo: dayInfo, SelectedSlot: selected, OrderForm: orderForm, OrderPreview: preview, CaptchaCheck: captchaCheck, CaptchaAnswer: captchaAnswer}, err
	}

	mergeTYYSCaptchaAnswerIntoValues(orderForm, captchaAnswer)
	if captchaCheck != nil {
		mergeTYYSJSONDataIntoValues(orderForm, captchaCheck.Data)
	}
	mergeURLValues(orderForm, req.SubmitExtra)

	submit, err := s.ReservationOrderSubmit(ctx, orderForm)
	return &TYYSReservationResult{
		VenueInfo:     venueInfo,
		DayInfo:       dayInfo,
		OrderPreview:  preview,
		CaptchaCheck:  captchaCheck,
		Submit:        submit,
		SelectedSlot:  selected,
		OrderForm:     orderForm,
		CaptchaAnswer: captchaAnswer,
	}, err
}

func (s *TYYS) resolveReservationVenue(ctx context.Context, req TYYSReservationRequest) (TYYSReservationRequest, *TYYSAPIResponse, error) {
	if strings.TrimSpace(req.VenueID) != "" && strings.TrimSpace(req.VenueSiteID) != "" {
		return req, nil, nil
	}
	venueInfo, err := s.VenueInfo(ctx, req.IsArt)
	if err != nil {
		return req, venueInfo, err
	}

	var payload any
	if err := json.Unmarshal(venueInfo.Data, &payload); err != nil {
		return req, venueInfo, err
	}

	var matched map[string]any
	walkJSONObjectsGeneric(payload, func(obj map[string]any) {
		if matched != nil || !objectLooksLikeVenueSite(obj) {
			return
		}
		if !reservationTextMatches(obj["sportName"], req.SportName) {
			return
		}
		if !reservationTextMatches(obj["campusName"], req.CampusName) {
			return
		}
		if !reservationTextMatches(obj["venueName"], req.VenueName) {
			return
		}
		matched = obj
	})
	if matched == nil {
		return req, venueInfo, fmt.Errorf("venue site not found for sport=%q campus=%q venue=%q", req.SportName, req.CampusName, req.VenueName)
	}
	req.VenueID = reservationString(matched["venueId"])
	req.VenueSiteID = reservationString(matched["id"])
	return req, venueInfo, nil
}

func objectLooksLikeVenueSite(obj map[string]any) bool {
	_, hasSiteName := obj["siteName"]
	_, hasVenueID := obj["venueId"]
	_, hasID := obj["id"]
	return hasSiteName && hasVenueID && hasID
}

func reservationTextMatches(value any, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return true
	}
	got := strings.TrimSpace(reservationString(value))
	return got == want || strings.Contains(got, want) || strings.Contains(want, got)
}

func (req TYYSReservationRequest) dayInfoParams() url.Values {
	values := url.Values{}
	values.Set("venueId", req.VenueID)
	values.Set("venueSiteId", req.VenueSiteID)
	values.Set("siteId", req.VenueSiteID)
	values.Set("date", req.Date)
	values.Set("reservationDate", req.Date)
	values.Set("searchDate", req.Date)
	if req.WeekStartDate != "" {
		values.Set("weekStartDate", req.WeekStartDate)
	}
	values.Set("startTime", req.StartTime)
	values.Set("endTime", req.EndTime)
	values.Set("isArt", fmt.Sprintf("%d", req.IsArt))
	mergeURLValues(values, req.DayInfoExtra)
	return values
}

func (req TYYSReservationRequest) orderInfoForm(selected map[string]any) url.Values {
	values := url.Values{}
	values.Set("venueSiteId", req.VenueSiteID)
	values.Set("reservationDate", req.Date)
	if req.WeekStartDate != "" {
		values.Set("weekStartDate", req.WeekStartDate)
	}
	item := map[string]any{
		"spaceId":           reservationString(selected["spaceId"]),
		"timeId":            reservationString(selected["timeId"]),
		"venueSpaceGroupId": selected["venueSpaceGroupId"],
	}
	values.Set("reservationOrderJson", mustMarshalJSON([]map[string]any{item}))
	return values
}

func selectTYYSReservationSlot(data json.RawMessage, req TYYSReservationRequest) (map[string]any, error) {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	var candidates []map[string]any
	walkJSONObjectsGeneric(payload, func(obj map[string]any) {
		if !reservationCourtMatches(obj, req) {
			return
		}
		slot, timeID, ok := reservationSlotAtTime(obj, req)
		if !ok || reservationSlotUnavailable(slot) {
			return
		}
		candidates = append(candidates, reservationSlotOrderFields(obj, slot, timeID))
	})
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no free slot found for %s %s-%s", req.Date, req.StartTime, req.EndTime)
	}
	return candidates[0], nil
}

func reservationCourtMatches(obj map[string]any, req TYYSReservationRequest) bool {
	if len(req.CourtNames) > 0 {
		name := strings.TrimSpace(reservationString(obj["spaceName"]))
		for _, want := range req.CourtNames {
			if strings.TrimSpace(want) == name {
				return true
			}
		}
		return false
	}
	if req.MinCourtNo != 0 || req.MaxCourtNo != 0 {
		no, ok := reservationObjectSpaceNo(obj)
		if !ok {
			return false
		}
		if req.MinCourtNo != 0 && no < req.MinCourtNo {
			return false
		}
		if req.MaxCourtNo != 0 && no > req.MaxCourtNo {
			return false
		}
	}
	return true
}

func reservationObjectSpaceNo(obj map[string]any) (int, bool) {
	for _, key := range []string{"spaceNo", "siteSpaceNo", "fieldNo", "fieldName", "spaceName", "name"} {
		if value, ok := obj[key]; ok {
			if number, ok := reservationInt(value); ok {
				return number, true
			}
			text := reservationString(value)
			for i := 1; i <= 99; i++ {
				if strings.Contains(text, strconv.Itoa(i)) {
					return i, true
				}
			}
		}
	}
	return 0, false
}

func reservationSlotAtTime(obj map[string]any, req TYYSReservationRequest) (map[string]any, string, bool) {
	for key, value := range obj {
		child, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if reservationString(child["startDate"]) == req.Date+" "+req.StartTime && reservationString(child["endDate"]) == req.Date+" "+req.EndTime {
			return child, key, true
		}
	}
	return nil, "", false
}

func reservationSlotUnavailable(obj map[string]any) bool {
	if status, ok := reservationInt(obj["reservationStatus"]); ok && status != 1 {
		return true
	}
	if count, ok := reservationInt(obj["alreadyNum"]); ok && count > 0 {
		return true
	}
	if tradeNo := strings.TrimSpace(reservationString(obj["tradeNo"])); tradeNo != "" && tradeNo != "null" {
		return true
	}
	return false
}

func reservationSlotOrderFields(space map[string]any, slot map[string]any, timeID string) map[string]any {
	fields := map[string]any{}
	for key, value := range slot {
		fields[key] = value
	}
	fields["timeId"] = timeID
	if value, ok := space["id"]; ok {
		fields["spaceId"] = value
		fields["venueSpaceId"] = value
		fields["venueSpaceIds"] = value
		fields["spaceInfoId"] = value
	}
	for _, key := range []string{"spaceName", "venueSpaceGroupId", "venueSiteId"} {
		if value, ok := space[key]; ok {
			fields[key] = value
		}
	}
	return fields
}

func (s *TYYS) addReservationBuddyFields(ctx context.Context, values url.Values, previewData json.RawMessage, req TYYSReservationRequest) error {
	buddyCode := strings.TrimSpace(req.BuddyCode)
	if buddyCode == "" {
		return nil
	}
	if values.Get("buddyNo") == "" {
		values.Set("buddyNo", buddyCode)
	}
	if buddyIDs := strings.TrimSpace(firstNonEmptyReservation(req.BuddyIDs, req.BuddyID)); buddyIDs != "" && values.Get("buddyIds") == "" {
		values.Set("buddyIds", buddyIDs)
	}
	if values.Get("buddyIds") == "" {
		buddyID, _, err := s.resolveReservationBuddyID(ctx, buddyCode, previewData)
		if err != nil {
			return fmt.Errorf("resolve buddy id for buddy code %s: %w", buddyCode, err)
		}
		values.Set("buddyIds", buddyID)
	}
	if values.Get("isOfflineTicket") == "" {
		values.Set("isOfflineTicket", firstNonEmptyReservation(req.IsOfflineTicket, "1"))
	}
	return nil
}

func (s *TYYS) resolveReservationBuddyID(ctx context.Context, buddyCode string, previewData json.RawMessage) (string, string, error) {
	if buddyID := findReservationBuddyID(previewData, buddyCode, false); buddyID != "" {
		return buddyID, "order_preview", nil
	}
	response, err := s.Buddies(ctx, nil)
	if err != nil {
		return "", "", err
	}
	if buddyID := findReservationBuddyID(response.Data, buddyCode, true); buddyID != "" {
		return buddyID, "buddies_api", nil
	}
	return "", "", fmt.Errorf("not found in order preview buddyList or /api/buddies")
}

func findReservationBuddyID(data json.RawMessage, buddyCode string, requireCodeMatch bool) string {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	var matchedID string
	walkJSONObjectsGeneric(payload, func(obj map[string]any) {
		if matchedID != "" {
			return
		}
		if requireCodeMatch && !reservationObjectMatchesBuddyCode(obj, buddyCode) {
			return
		}
		if !requireCodeMatch && !reservationObjectLooksLikeBuddy(obj) {
			return
		}
		matchedID = reservationObjectBuddyID(obj)
	})
	return matchedID
}

func reservationObjectLooksLikeBuddy(obj map[string]any) bool {
	for _, key := range []string{"userUid", "userPhone", "userRoleId"} {
		if _, ok := obj[key]; ok {
			return true
		}
	}
	return false
}

func reservationObjectMatchesBuddyCode(obj map[string]any, buddyCode string) bool {
	for _, key := range []string{"buddyNo", "buddyCode", "userCode", "code", "userNo", "no"} {
		if strings.TrimSpace(reservationString(obj[key])) == buddyCode {
			return true
		}
	}
	return false
}

func reservationObjectBuddyID(obj map[string]any) string {
	for _, key := range []string{"id", "buddyId", "userId", "uid"} {
		value := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(reservationString(obj[key]), ".0"), ".00"))
		if value != "" && value != "0" && value != "null" {
			return value
		}
	}
	return ""
}

func mergeTYYSOrderPreviewSubmitFields(dst url.Values, data json.RawMessage, req TYYSReservationRequest) {
	if strings.TrimSpace(req.Phone) != "" {
		dst.Set("phone", strings.TrimSpace(req.Phone))
		return
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return
	}
	if value, ok := obj["phone"]; ok && value != nil {
		dst.Set("phone", reservationString(value))
	}
}

func requireTYYSCaptchaCheckOK(response *TYYSAPIResponse) error {
	if response == nil || len(response.Data) == 0 {
		return fmt.Errorf("captcha check did not return data")
	}
	var data struct {
		RepCode string `json:"repCode"`
		RepMsg  string `json:"repMsg"`
	}
	if err := json.Unmarshal(response.Data, &data); err != nil {
		return fmt.Errorf("parse captcha check data: %w", err)
	}
	if data.RepCode != "0000" {
		return fmt.Errorf("captcha check failed: repCode=%s repMsg=%s", data.RepCode, data.RepMsg)
	}
	return nil
}

func mergeTYYSCaptchaAnswerIntoValues(dst url.Values, answer url.Values) {
	if value := answer.Get("captchaVerification"); value != "" {
		dst.Set("captchaVerification", value)
	}
}

func mergeTYYSJSONDataIntoValues(dst url.Values, data json.RawMessage) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return
	}
	for _, key := range []string{"captchaVerification", "pointJson"} {
		if value, ok := obj[key]; ok && value != nil {
			dst.Set(key, reservationString(value))
		}
	}
	if value, ok := obj["repData"]; ok {
		mergeTYYSCaptchaRepDataIntoValues(dst, value)
	}
}

func mergeTYYSCaptchaRepDataIntoValues(dst url.Values, value any) {
	switch item := value.(type) {
	case string:
		if item != "" {
			dst.Set("captchaVerification", item)
		}
	case map[string]any:
		for _, key := range []string{"captchaVerification", "pointJson"} {
			if nested, ok := item[key]; ok && nested != nil {
				dst.Set(key, reservationString(nested))
			}
		}
	}
}

func mergeTYYSReservationMetadata(dst url.Values, data json.RawMessage, req TYYSReservationRequest) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return
	}
	for _, key := range []string{"token", "weekStartDate"} {
		if value, ok := obj[key]; ok {
			dst.Set(key, reservationString(value))
		}
	}
	if dst.Get("weekStartDate") == "" && req.WeekStartDate != "" {
		dst.Set("weekStartDate", req.WeekStartDate)
	}
}

func walkJSONObjectsGeneric(value any, visit func(map[string]any)) {
	switch typed := value.(type) {
	case map[string]any:
		visit(typed)
		for _, child := range typed {
			walkJSONObjectsGeneric(child, visit)
		}
	case []any:
		for _, child := range typed {
			walkJSONObjectsGeneric(child, visit)
		}
	}
}

func reservationInt(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return parsed, err == nil
	default:
		return 0, false
	}
}

func reservationString(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	var text string
	if err := json.Unmarshal(bytes, &text); err == nil {
		return text
	}
	return strings.Trim(string(bytes), `"`)
}

func mustMarshalJSON(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(bytes)
}

func mergeURLValues(dst url.Values, src url.Values) {
	for key, values := range src {
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func reservationRandomUUIDLikeString() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func firstNonEmptyReservation(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
