package zjulogin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	tyysBadmintonDate          = "2026-04-12"
	tyysBadmintonWeekStartDate = "2026-04-12"
	tyysBadmintonStartTime     = "13:30"
	tyysBadmintonEndTime       = "14:30"
)

type timingEntry struct {
	Label    string
	Elapsed  time.Duration
	ErrorMsg string
}

type timingRecorder struct {
	t       *testing.T
	entries []timingEntry
}

func TestTYYSZijingangBadmintonReservationFlow(t *testing.T) {
	if os.Getenv("ZJU_LOGIN_LIVE_TEST") != "1" {
		t.Skip("set ZJU_LOGIN_LIVE_TEST=1 and configure .env.zju to run this live TYYS flow")
	}
	if os.Getenv("TYYS_LIVE_SUBMIT") != "1" {
		t.Skip("set TYYS_LIVE_SUBMIT=1 to run the real TYYS reservation submit flow")
	}
	if strings.TrimSpace(os.Getenv("TYYS_BUDDY_CODE")) == "" && strings.TrimSpace(os.Getenv("TYYS_BADMINTON_ORDER_EXTRA_JSON")) == "" {
		t.Fatal("TYYS_BUDDY_CODE or TYYS_BADMINTON_ORDER_EXTRA_JSON is required for badminton live submit")
	}

	timings := &timingRecorder{t: t}
	defer timings.LogSummary()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	auth, err := NewFromEnv()
	if err != nil {
		t.Fatalf("new auth from .env.zju: %v", err)
	}
	tyys, err := timedCall(t, timings, "new_tyys_service", func() (*TYYS, error) {
		return auth.TYYS()
	})
	if err != nil {
		t.Fatalf("new tyys service: %v", err)
	}

	solver := TYYSPythonCaptchaSolver{
		PythonPath: firstNonEmpty(valuesFromEnv("TYYS_CAPTCHA_PYTHON"), "python"),
		ScriptPath: firstNonEmpty(valuesFromEnv("TYYS_CAPTCHA_SCRIPT"), tyysDefaultCaptchaScript),
		ExtraArgs:  tyysCaptchaSolverExtraArgs("tyys_badminton_captcha_annotated.png"),
	}

	result, err := timedCall(t, timings, "badminton_reserve_flow", func() (*TYYSReservationResult, error) {
		return tyys.Reserve(ctx, TYYSReservationRequest{
			SportName:       "羽毛球",
			CampusName:      "紫金港校区",
			VenueName:       "风雨操场",
			Date:            tyysBadmintonDate,
			WeekStartDate:   tyysBadmintonWeekStartDate,
			StartTime:       tyysBadmintonStartTime,
			EndTime:         tyysBadmintonEndTime,
			MinCourtNo:      1,
			MaxCourtNo:      5,
			BuddyCode:       strings.TrimSpace(os.Getenv("TYYS_BUDDY_CODE")),
			BuddyID:         strings.TrimSpace(os.Getenv("TYYS_BUDDY_ID")),
			BuddyIDs:        strings.TrimSpace(os.Getenv("TYYS_BUDDY_IDS")),
			Phone:           strings.TrimSpace(os.Getenv("TYYS_PHONE")),
			IsOfflineTicket: strings.TrimSpace(os.Getenv("TYYS_IS_OFFLINE_TICKET")),
			CaptchaSolver:   solver,
			DayInfoExtra:    valuesFromEnvJSON("TYYS_BADMINTON_DAY_INFO_PARAMS_JSON"),
			OrderInfoExtra:  valuesFromEnvJSON("TYYS_BADMINTON_ORDER_EXTRA_JSON"),
			SubmitExtra:     valuesFromEnvJSON("TYYS_BADMINTON_SUBMIT_EXTRA_JSON"),
			CaptchaParams:   valuesFromEnvJSON("TYYS_CAPTCHA_GET_PARAMS_JSON"),
		})
	})
	if result != nil {
		t.Logf("selected badminton slot=%s", mustMarshalForLog(result.SelectedSlot))
		t.Logf("badminton order form=%s", result.OrderForm.Encode())
		logTYYSPayload(t, "badminton_day_info", result.DayInfo, nil)
		logTYYSPayload(t, "badminton_order_preview", result.OrderPreview, nil)
		logTYYSPayload(t, "badminton_captcha_check", result.CaptchaCheck, nil)
		logTYYSPayload(t, "badminton_order_submit", result.Submit, err)
	}
	if err != nil {
		t.Fatalf("badminton reserve: %v", err)
	}
}

func tyysCaptchaSolverExtraArgs(defaultAnnotatePath string) []string {
	annotatePath := firstNonEmpty(valuesFromEnv("TYYS_CAPTCHA_ANNOTATE"), defaultAnnotatePath)
	var args []string
	if annotatePath != "" {
		args = append(args, "--annotate", annotatePath)
	}
	if mode := strings.TrimSpace(os.Getenv("TYYS_CAPTCHA_AES_MODE")); mode != "" {
		args = append(args, "--aes-mode", mode)
	}
	if pointsJSON := strings.TrimSpace(os.Getenv("TYYS_CAPTCHA_POINTS_JSON")); pointsJSON != "" {
		args = append(args, "--points-json", pointsJSON)
	}
	return args
}

func valuesFromEnvJSON(name string) url.Values {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}
	values := url.Values{}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return values
	}
	for key, value := range obj {
		switch item := value.(type) {
		case string:
			values.Set(key, item)
		case float64, bool:
			values.Set(key, strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(reservationString(item), ".0"), ".00")))
		case []any:
			for _, elem := range item {
				values.Add(key, reservationString(elem))
			}
		}
	}
	return values
}

func logTYYSPayload(t *testing.T, label string, payload *TYYSAPIResponse, err error) {
	t.Helper()
	if payload == nil {
		t.Logf("%s error=%v", label, err)
		return
	}
	body, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		t.Logf("%s code=%d message=%q marshal_error=%v original_error=%v", label, payload.Code, payload.Message, marshalErr, err)
		return
	}
	t.Logf("%s response=%s error=%v", label, string(body), err)
}

func timedCall[T any](t *testing.T, timings *timingRecorder, label string, fn func() (T, error)) (T, error) {
	t.Helper()
	start := time.Now()
	value, err := fn()
	timings.Record(label, time.Since(start), err)
	return value, err
}

func (r *timingRecorder) Record(label string, elapsed time.Duration, err error) {
	entry := timingEntry{
		Label:   label,
		Elapsed: elapsed.Round(time.Millisecond),
	}
	if err != nil {
		entry.ErrorMsg = err.Error()
	}
	r.entries = append(r.entries, entry)
}

func (r *timingRecorder) LogSummary() {
	r.t.Helper()
	if len(r.entries) == 0 {
		return
	}
	var builder strings.Builder
	builder.WriteString("timing summary:")
	for _, entry := range r.entries {
		builder.WriteString("\n  ")
		builder.WriteString(entry.Label)
		builder.WriteString("=")
		builder.WriteString(entry.Elapsed.String())
		if entry.ErrorMsg != "" {
			builder.WriteString(" error=")
			builder.WriteString(entry.ErrorMsg)
		}
	}
	r.t.Log(builder.String())
}

func mustMarshalForLog(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(bytes)
}

func valuesFromEnv(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
