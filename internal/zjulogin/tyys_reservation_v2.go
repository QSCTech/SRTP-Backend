package zjulogin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TYYSReservationV2Request is the execute-only reservation request shape.
//
// Callers must provide an already-selected slot via reservation_date +
// venue_site_id + space_id + time_id. This flow does not call day/info and
// does not try to infer venue, time window, or court selection.
type TYYSReservationV2Request struct {
	ReservationDate string
	WeekStartDate   string
	Token           string

	VenueSiteID string
	SpaceID     string
	TimeID      string

	BuddyCode       string
	BuddyID         string
	BuddyIDs        string
	Phone           string
	IsOfflineTicket string

	CaptchaSolver TYYSCaptchaSolver

	OrderInfoExtra url.Values
	SubmitExtra    url.Values
	CaptchaParams  url.Values
}

type TYYSReservationV2Result struct {
	ReservationDate string
	VenueSiteID     string
	SpaceID         string
	TimeID          string
	Token           string

	OrderForm     url.Values
	CaptchaAnswer url.Values

	OrderPreview *TYYSAPIResponse
	CaptchaCheck *TYYSAPIResponse
	Submit       *TYYSAPIResponse
}

func (s *TYYS) ReserveV2(ctx context.Context, req TYYSReservationV2Request) (*TYYSReservationV2Result, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}

	orderForm := req.orderInfoForm()
	mergeURLValues(orderForm, req.OrderInfoExtra)

	preview, err := s.ReservationOrderInfo(ctx, orderForm)
	if err != nil {
		return &TYYSReservationV2Result{
			ReservationDate: req.ReservationDate,
			VenueSiteID:     req.VenueSiteID,
			SpaceID:         req.SpaceID,
			TimeID:          req.TimeID,
			Token:           req.Token,
			OrderForm:       orderForm,
			OrderPreview:    preview,
		}, err
	}

	mergeTYYSOrderPreviewSubmitFieldsV2(orderForm, preview.Data, req)
	if err := s.addReservationBuddyFieldsV2(ctx, orderForm, preview.Data, req); err != nil {
		return &TYYSReservationV2Result{
			ReservationDate: req.ReservationDate,
			VenueSiteID:     req.VenueSiteID,
			SpaceID:         req.SpaceID,
			TimeID:          req.TimeID,
			Token:           req.Token,
			OrderForm:       orderForm,
			OrderPreview:    preview,
		}, err
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
		return &TYYSReservationV2Result{
			ReservationDate: req.ReservationDate,
			VenueSiteID:     req.VenueSiteID,
			SpaceID:         req.SpaceID,
			TimeID:          req.TimeID,
			Token:           req.Token,
			OrderForm:       orderForm,
			OrderPreview:    preview,
			CaptchaCheck:    captchaCheck,
			CaptchaAnswer:   captchaAnswer,
		}, err
	}
	if err := requireTYYSCaptchaCheckOK(captchaCheck); err != nil {
		return &TYYSReservationV2Result{
			ReservationDate: req.ReservationDate,
			VenueSiteID:     req.VenueSiteID,
			SpaceID:         req.SpaceID,
			TimeID:          req.TimeID,
			Token:           req.Token,
			OrderForm:       orderForm,
			OrderPreview:    preview,
			CaptchaCheck:    captchaCheck,
			CaptchaAnswer:   captchaAnswer,
		}, err
	}

	mergeTYYSCaptchaAnswerIntoValues(orderForm, captchaAnswer)
	if captchaCheck != nil {
		mergeTYYSJSONDataIntoValues(orderForm, captchaCheck.Data)
	}
	mergeURLValues(orderForm, req.SubmitExtra)

	submit, err := s.ReservationOrderSubmit(ctx, orderForm)
	return &TYYSReservationV2Result{
		ReservationDate: req.ReservationDate,
		VenueSiteID:     req.VenueSiteID,
		SpaceID:         req.SpaceID,
		TimeID:          req.TimeID,
		Token:           req.Token,
		OrderForm:       orderForm,
		CaptchaAnswer:   captchaAnswer,
		OrderPreview:    preview,
		CaptchaCheck:    captchaCheck,
		Submit:          submit,
	}, err
}

func (req TYYSReservationV2Request) validate() error {
	if strings.TrimSpace(req.ReservationDate) == "" {
		return fmt.Errorf("reservation date is required")
	}
	if strings.TrimSpace(req.VenueSiteID) == "" {
		return fmt.Errorf("venue site id is required")
	}
	if strings.TrimSpace(req.SpaceID) == "" {
		return fmt.Errorf("space id is required")
	}
	if strings.TrimSpace(req.TimeID) == "" {
		return fmt.Errorf("time id is required")
	}
	if strings.TrimSpace(req.Token) == "" {
		return fmt.Errorf("token is required")
	}
	if req.CaptchaSolver == nil {
		return fmt.Errorf("captcha solver is required")
	}
	return nil
}

func (req TYYSReservationV2Request) orderInfoForm() url.Values {
	values := url.Values{}
	values.Set("venueSiteId", strings.TrimSpace(req.VenueSiteID))
	values.Set("reservationDate", strings.TrimSpace(req.ReservationDate))
	values.Set("weekStartDate", req.normalizedWeekStartDate())
	values.Set("token", strings.TrimSpace(req.Token))
	item := map[string]any{
		"spaceId":           strings.TrimSpace(req.SpaceID),
		"timeId":            strings.TrimSpace(req.TimeID),
		"venueSpaceGroupId": nil,
	}
	values.Set("reservationOrderJson", mustMarshalJSON([]map[string]any{item}))
	return values
}

func (req TYYSReservationV2Request) normalizedWeekStartDate() string {
	if weekStartDate := strings.TrimSpace(req.WeekStartDate); weekStartDate != "" {
		return weekStartDate
	}
	return strings.TrimSpace(req.ReservationDate)
}

func (s *TYYS) addReservationBuddyFieldsV2(ctx context.Context, values url.Values, previewData json.RawMessage, req TYYSReservationV2Request) error {
	legacy := TYYSReservationRequest{
		BuddyCode:       req.BuddyCode,
		BuddyID:         req.BuddyID,
		BuddyIDs:        req.BuddyIDs,
		IsOfflineTicket: req.IsOfflineTicket,
	}
	return s.addReservationBuddyFields(ctx, values, previewData, legacy)
}

func mergeTYYSOrderPreviewSubmitFieldsV2(dst url.Values, data json.RawMessage, req TYYSReservationV2Request) {
	legacy := TYYSReservationRequest{
		Phone: req.Phone,
	}
	mergeTYYSOrderPreviewSubmitFields(dst, data, legacy)
}
