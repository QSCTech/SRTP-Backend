package zjulogin

import (
	"context"
	"io"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestTYYSSignMatchesBrowserRequest(t *testing.T) {
	params := url.Values{}
	params.Set("isArt", "0")
	params.Set("nocache", "1775791458379")

	sign := tyysSign(
		tyysDefaultSignSecret,
		tyysSignPath("/venue-server/api/reservation/campus/venue/info"),
		params,
		"1775791458379",
	)

	if sign != "08387b2883176726c3951fb961c353a6" {
		t.Fatalf("sign=%s", sign)
	}
}

func TestTYYSGetVenueInfo(t *testing.T) {
	if os.Getenv("ZJU_LOGIN_LIVE_TEST") != "1" {
		t.Skip("set ZJU_LOGIN_LIVE_TEST=1 and configure .env.zju to run this live login test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	auth, err := NewFromEnv()
	if err != nil {
		t.Fatalf("new auth from .env.zju: %v", err)
	}

	tyys, err := auth.TYYS()
	if err != nil {
		t.Fatalf("new tyys service: %v", err)
	}

	res, err := tyys.Get(ctx, tyys.VenueInfoURL(0))
	if err != nil {
		t.Fatalf("get venue info: %v", err)
	}
	defer closeResponseBody(res)

	body, err := io.ReadAll(io.LimitReader(res.Body, 4096))
	if err != nil {
		t.Fatalf("read venue info response: %v", err)
	}

	t.Logf("status=%d body_prefix=%s", res.StatusCode, string(body))
}
