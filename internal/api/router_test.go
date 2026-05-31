package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QSCTech/SRTP-Backend/internal/api/gen"
	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/internal/service"
	"github.com/QSCTech/SRTP-Backend/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRouterAuthAndProfileFlow(t *testing.T) {
	router, db := newTestRouter(t)

	requestBody := []byte(`{"auth_uid":"auth-1","code":"open-1"}`)
	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, jsonRequest(http.MethodPost, "/auth/wx/login", requestBody))
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginRecorder.Code, loginRecorder.Body.String())
	}
	var loginResponse gen.UserResponse
	decodeJSON(t, loginRecorder.Body, &loginResponse)
	if _, err := uuid.Parse(loginResponse.PublicId.String()); err != nil {
		t.Fatalf("login public_id=%q is not a valid UUID: %v", loginResponse.PublicId.String(), err)
	}
	if loginResponse.ProfileStatus != "pending" {
		t.Fatalf("login profile_status=%q want pending", loginResponse.ProfileStatus)
	}

	duplicateLoginRecorder := httptest.NewRecorder()
	router.ServeHTTP(duplicateLoginRecorder, jsonRequest(http.MethodPost, "/auth/wx/login", requestBody))
	if duplicateLoginRecorder.Code != http.StatusOK {
		t.Fatalf("duplicate login status=%d body=%s", duplicateLoginRecorder.Code, duplicateLoginRecorder.Body.String())
	}

	var userCount int64
	if err := db.Model(&models.User{}).Where("auth_uid = ?", "auth-1").Count(&userCount).Error; err != nil {
		t.Fatalf("count login user: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("auth-1 user count=%d want 1", userCount)
	}

	var user models.User
	if err := db.Where("auth_uid = ?", "auth-1").First(&user).Error; err != nil {
		t.Fatalf("find login user: %v", err)
	}

	getUserRecorder := httptest.NewRecorder()
	router.ServeHTTP(getUserRecorder, httptest.NewRequest(http.MethodGet, "/users/"+strconv.FormatUint(uint64(user.ID), 10), nil))
	if getUserRecorder.Code != http.StatusOK {
		t.Fatalf("get user status=%d body=%s", getUserRecorder.Code, getUserRecorder.Body.String())
	}

	missingUserRecorder := httptest.NewRecorder()
	router.ServeHTTP(missingUserRecorder, httptest.NewRequest(http.MethodGet, "/users/999999", nil))
	if missingUserRecorder.Code != http.StatusNotFound {
		t.Fatalf("missing user status=%d body=%s", missingUserRecorder.Code, missingUserRecorder.Body.String())
	}

	meRecorder := httptest.NewRecorder()
	router.ServeHTTP(meRecorder, httptest.NewRequest(http.MethodGet, "/me", nil))
	if meRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized /me status=%d body=%s", meRecorder.Code, meRecorder.Body.String())
	}

	meRecorder = httptest.NewRecorder()
	meRequest := httptest.NewRequest(http.MethodGet, "/me", nil)
	meRequest.Header.Set("X-Auth-UID", "auth-1")
	router.ServeHTTP(meRecorder, meRequest)
	if meRecorder.Code != http.StatusOK {
		t.Fatalf("authorized /me status=%d body=%s", meRecorder.Code, meRecorder.Body.String())
	}

	profileRecorder := httptest.NewRecorder()
	profileRequest := jsonRequest(http.MethodPut, "/me/profile", []byte(`{"nickname":"new nick"}`))
	profileRequest.Header.Set("X-Auth-UID", "auth-1")
	router.ServeHTTP(profileRecorder, profileRequest)
	if profileRecorder.Code != http.StatusOK {
		t.Fatalf("profile status=%d body=%s", profileRecorder.Code, profileRecorder.Body.String())
	}

	var updated models.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("find updated user: %v", err)
	}
	if updated.Nickname != "new nick" {
		t.Fatalf("Nickname=%q want new nick", updated.Nickname)
	}
	if updated.ProfileStatus != "pending_review" {
		t.Fatalf("ProfileStatus=%q want pending_review", updated.ProfileStatus)
	}

	var auditCount int64
	if err := db.Model(&models.UserProfileAudit{}).Where("user_id = ?", user.ID).Count(&auditCount).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit count=%d want 1", auditCount)
	}
}

func TestRouterAllowsUnauthenticatedRoomList(t *testing.T) {
	router, _ := newTestRouter(t)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/rooms", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("/rooms status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func newTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.UserProfileAudit{}, &models.Room{}, &models.RoomMember{}, &models.JoinRequest{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	roomService := service.NewRoomService(repository.NewRoomRepository(db), userService)
	reservationService := service.NewReservationService(repository.NewRoomRepository(db), repository.NewReservationRepository(db))
	return NewRouter(zap.NewNop(), sqlDB, userService, roomService, reservationService), db
}

func jsonRequest(method, target string, body []byte) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func decodeJSON(t *testing.T, body *bytes.Buffer, target any) {
	t.Helper()
	if err := json.NewDecoder(body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
