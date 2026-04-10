//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	zjulogin "github.com/QSCTech/SRTP-Backend/internal/zjulogin"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	auth, err := zjulogin.NewFromEnv()
	if err != nil {
		panic(err)
	}
	tyys, err := auth.TYYS()
	if err != nil {
		panic(err)
	}

	solver := zjulogin.TYYSPythonCaptchaSolver{
		PythonPath: firstNonEmpty(os.Getenv("TYYS_CAPTCHA_PYTHON"), "python"),
		ScriptPath: firstNonEmpty(os.Getenv("TYYS_CAPTCHA_SCRIPT"), "scripts/tyys_captcha_solver.py"),
	}

	result, err := tyys.Reserve(ctx, zjulogin.TYYSReservationRequest{
		SportName:       "\u7fbd\u6bdb\u7403",
		CampusName:      "\u7d2b\u91d1\u6e2f\u6821\u533a",
		VenueName:       "\u98ce\u96e8\u64cd\u573a",
		Date:            "2026-04-12",
		WeekStartDate:   "2026-04-12",
		StartTime:       "13:30",
		EndTime:         "14:30",
		MinCourtNo:      1,
		MaxCourtNo:      5,
		BuddyCode:       "xxxxx", //同伴码
		Phone:           "xxxxxxx",
		IsOfflineTicket: firstNonEmpty(os.Getenv("TYYS_IS_OFFLINE_TICKET"), "1"),
		CaptchaSolver:   solver,
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("selected slot: %+v\n", result.SelectedSlot)
	if result.Submit != nil {
		fmt.Printf("submit code=%d message=%s\n", result.Submit.Code, result.Submit.Message)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
