package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/QSCTech/SRTP-Backend/internal/middleware"
	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserServiceCreate(t *testing.T) {
	service, db := newTestUserService(t)

	user, err := service.Create(context.Background(), "auth-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if user.AuthUID != "auth-1" {
		t.Fatalf("AuthUID=%q want %q", user.AuthUID, "auth-1")
	}
	if user.ProfileStatus != "pending" {
		t.Fatalf("ProfileStatus=%q want %q", user.ProfileStatus, "pending")
	}
	if _, err := uuid.Parse(user.PublicID); err != nil {
		t.Fatalf("PublicID=%q is not a valid UUID: %v", user.PublicID, err)
	}

	if _, err := service.Create(context.Background(), "auth-1"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate Create error=%v want already exists", err)
	}

	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("user count=%d want 1", count)
	}
}

func TestUserServiceGetByID(t *testing.T) {
	service, _ := newTestUserService(t)
	created, err := service.Create(context.Background(), "auth-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := service.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if found.ID != created.ID {
		t.Fatalf("ID=%d want %d", found.ID, created.ID)
	}

	_, err = service.GetByID(context.Background(), created.ID+1)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing GetByID error=%v want gorm.ErrRecordNotFound", err)
	}
}

func TestUserServiceGetCurrent(t *testing.T) {
	service, _ := newTestUserService(t)
	created, err := service.Create(context.Background(), "auth-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = service.GetCurrent(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unauthorized error=%v want ErrUnauthorized", err)
	}

	ctx := context.WithValue(context.Background(), middleware.AuthUIDKey, "missing-auth")
	_, err = service.GetCurrent(ctx)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("missing user error=%v want ErrUserNotFound", err)
	}

	ctx = context.WithValue(context.Background(), middleware.AuthUIDKey, created.AuthUID)
	found, err := service.GetCurrent(ctx)
	if err != nil {
		t.Fatalf("GetCurrent: %v", err)
	}
	if found.ID != created.ID {
		t.Fatalf("ID=%d want %d", found.ID, created.ID)
	}
}

func TestUserServiceUpdateCurrentProfileCreatesAudit(t *testing.T) {
	service, db := newTestUserService(t)
	user, err := service.Create(context.Background(), "auth-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	user.Bio = "old bio"
	user.ProfileStatus = "approved"
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("seed approved user: %v", err)
	}

	ctx := context.WithValue(context.Background(), middleware.AuthUIDKey, user.AuthUID)
	nickname := "new nick"
	updated, err := service.UpdateCurrentProfile(ctx, UpdateProfileInput{Nickname: &nickname})
	if err != nil {
		t.Fatalf("UpdateCurrentProfile nickname: %v", err)
	}
	if updated.Nickname != nickname {
		t.Fatalf("Nickname=%q want %q", updated.Nickname, nickname)
	}
	if updated.Bio != "old bio" {
		t.Fatalf("Bio=%q want preserved old bio", updated.Bio)
	}
	if updated.ProfileStatus != "pending_review" {
		t.Fatalf("ProfileStatus=%q want pending_review", updated.ProfileStatus)
	}
	assertAuditCount(t, db, user.ID, 1)

	bio := "new bio"
	updated, err = service.UpdateCurrentProfile(ctx, UpdateProfileInput{Bio: &bio})
	if err != nil {
		t.Fatalf("UpdateCurrentProfile bio: %v", err)
	}
	if updated.Nickname != nickname {
		t.Fatalf("Nickname=%q want preserved %q", updated.Nickname, nickname)
	}
	if updated.Bio != bio {
		t.Fatalf("Bio=%q want %q", updated.Bio, bio)
	}
	if updated.ProfileStatus != "pending_review" {
		t.Fatalf("ProfileStatus=%q want pending_review", updated.ProfileStatus)
	}
	assertAuditCount(t, db, user.ID, 2)
}

func TestUserServiceLoginOrCreate(t *testing.T) {
	service, db := newTestUserService(t)

	created, err := service.LoginOrCreate(context.Background(), "auth-1", "open-1")
	if err != nil {
		t.Fatalf("LoginOrCreate new: %v", err)
	}
	if created.OpenID != "open-1" {
		t.Fatalf("OpenID=%q want open-1", created.OpenID)
	}
	if created.ProfileStatus != "pending" {
		t.Fatalf("ProfileStatus=%q want pending", created.ProfileStatus)
	}
	if _, err := uuid.Parse(created.PublicID); err != nil {
		t.Fatalf("PublicID=%q is not a valid UUID: %v", created.PublicID, err)
	}

	existing, err := service.LoginOrCreate(context.Background(), "auth-1", "open-2")
	if err != nil {
		t.Fatalf("LoginOrCreate existing: %v", err)
	}
	if existing.ID != created.ID {
		t.Fatalf("ID=%d want existing ID %d", existing.ID, created.ID)
	}
	if existing.OpenID != "open-2" {
		t.Fatalf("OpenID=%q want updated open-2", existing.OpenID)
	}

	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("user count=%d want 1", count)
	}
}

func newTestUserService(t *testing.T) (*UserService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.UserProfileAudit{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return NewUserService(repository.NewUserRepository(db)), db
}

func assertAuditCount(t *testing.T, db *gorm.DB, userID uint, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&models.UserProfileAudit{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if count != want {
		t.Fatalf("audit count=%d want %d", count, want)
	}
}
