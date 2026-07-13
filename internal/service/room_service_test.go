package service

import (
	"context"
	"testing"
	"time"

	"github.com/QSCTech/SRTP-Backend/internal/middleware"
	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRoomServiceCreateRetriesInviteCodeConflict(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Room{}, &models.RoomMember{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	user := models.User{AuthUID: "owner-auth", ProfileStatus: "approved"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	existing := models.Room{
		OwnerID: user.ID, Name: "existing", SportType: "badminton",
		CampusName: "Zijingang", VenueName: "Gym", Visibility: "public",
		JoinMode: "direct", Status: "recruiting", ReservationStatus: "not_required",
		ReservationProvider: "tyys", StartTime: time.Now().Add(time.Hour),
		EndTime: time.Now().Add(2 * time.Hour), InviteCode: "COLLIDE1",
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing room: %v", err)
	}

	userService := NewUserService(repository.NewUserRepository(db))
	roomService := NewRoomService(repository.NewRoomRepository(db), userService)
	codes := []string{"COLLIDE1", "UNIQUE02"}
	calls := 0
	roomService.inviteCodeGenerator = func() (string, error) {
		code := codes[calls]
		calls++
		return code, nil
	}
	ctx := context.WithValue(context.Background(), middleware.AuthUIDKey, user.AuthUID)
	limit := int32(2)
	created, err := roomService.Create(ctx, CreateRoomInput{
		Name: "new room", SportType: "badminton", CampusName: "Zijingang",
		VenueName: "Gym", Visibility: "public", JoinMode: "direct",
		StartTime: time.Now().Add(3 * time.Hour), EndTime: time.Now().Add(4 * time.Hour),
		MemberLimit: &limit,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if calls != 2 {
		t.Fatalf("invite code generator calls=%d want 2", calls)
	}
	if created.InviteCode != "UNIQUE02" {
		t.Fatalf("InviteCode=%q want UNIQUE02", created.InviteCode)
	}

	var ownerMemberCount int64
	if err := db.Model(&models.RoomMember{}).
		Where("room_id = ? AND user_id = ? AND role = ? AND status = ?", created.ID, user.ID, "owner", "joined").
		Count(&ownerMemberCount).Error; err != nil {
		t.Fatalf("count owner member: %v", err)
	}
	if ownerMemberCount != 1 {
		t.Fatalf("owner member count=%d want 1", ownerMemberCount)
	}
}
