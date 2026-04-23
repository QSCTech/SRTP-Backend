package zjulogin

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
)

func TestTYYSReservationV2OrderInfoForm(t *testing.T) {
	req := TYYSReservationV2Request{
		ReservationDate: "2026-04-23",
		Token:           "test-token",
		VenueSiteID:     "143",
		SpaceID:         "322",
		TimeID:          "21992",
	}

	form := req.orderInfoForm()
	if got, want := form.Get("venueSiteId"), "143"; got != want {
		t.Fatalf("venueSiteId=%q want %q", got, want)
	}
	if got, want := form.Get("reservationDate"), "2026-04-23"; got != want {
		t.Fatalf("reservationDate=%q want %q", got, want)
	}
	if got, want := form.Get("weekStartDate"), "2026-04-23"; got != want {
		t.Fatalf("weekStartDate=%q want %q", got, want)
	}
	if got, want := form.Get("token"), "test-token"; got != want {
		t.Fatalf("token=%q want %q", got, want)
	}
	if got, want := form.Get("reservationOrderJson"), `[{"spaceId":"322","timeId":"21992","venueSpaceGroupId":null}]`; got != want {
		t.Fatalf("reservationOrderJson=%q want %q", got, want)
	}
}

func TestTYYSReservationV2OrderInfoFormKeepsExplicitWeekStartDate(t *testing.T) {
	req := TYYSReservationV2Request{
		ReservationDate: "2026-04-23",
		WeekStartDate:   "2026-04-21",
		Token:           "test-token",
		VenueSiteID:     "143",
		SpaceID:         "322",
		TimeID:          "21992",
	}

	form := req.orderInfoForm()
	if got, want := form.Get("weekStartDate"), "2026-04-21"; got != want {
		t.Fatalf("weekStartDate=%q want %q", got, want)
	}
}

func TestTYYSReservationV2Validate(t *testing.T) {
	req := TYYSReservationV2Request{
		ReservationDate: "2026-04-23",
		Token:           "test-token",
		VenueSiteID:     "143",
		SpaceID:         "322",
		TimeID:          "21992",
		CaptchaSolver:   testCaptchaSolver(url.Values{"token": {"x"}, "pointJson": {"y"}}),
	}
	if err := req.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

type testCaptchaSolver url.Values

func (s testCaptchaSolver) SolveTYYS(_ context.Context, _ json.RawMessage) (url.Values, error) {
	return url.Values(s), nil
}
