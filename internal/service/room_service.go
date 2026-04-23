package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"
)

type RoomService struct {
	repo        *repository.RoomRepository
	userService *UserService
}

func NewRoomService(repo *repository.RoomRepository, userService *UserService) *RoomService {
	return &RoomService{repo: repo, userService: userService}
}

type ListRoomsInput struct {
	Keyword      *string
	SportType    *string
	Campus       *string
	Date         *time.Time
	TimeRange    *string
	Organization *string
	Level        *string
	Page         int
	PageSize     int
}

type CreateRoomInput struct {
	Name            string
	SportType       string
	CampusName      string
	VenueName       string
	Visibility      string
	JoinMode        string
	StartTime       time.Time
	EndTime         time.Time
	NeedReservation bool
	GenderRule      *string
	MemberLimit     *int32
	Organization    *string
	LevelDesc       *string
	Description     *string
}

type UpdateRoomInput struct {
	Name            *string
	Visibility      *string
	JoinMode        *string
	StartTime       *time.Time
	EndTime         *time.Time
	NeedReservation *bool
	GenderRule      *string
	MemberLimit     *int32
	Organization    *string
	LevelDesc       *string
	Description     *string
}

type JoinRoomByCodeInput struct {
	InviteCode string
}

type CreateJoinRequestInput struct {
	Message string
}

type ReviewJoinRequestInput struct {
	RequestID uint
}

type InviteMemberInput struct {
	UserID uint
}

type RoomCardItem struct {
	Room               models.Room
	CurrentMemberCount int32
}

type ListRoomsOutput struct {
	Page     int32
	PageSize int32
	Total    int64
	Items    []RoomCardItem
}

type JoinRoomOutput struct {
	RoomID        uint
	JoinResult    string
	MemberStatus  *string
	RequestStatus *string
}

type UserStatsOutput struct {
	CreatedRoomCount        int64
	JoinedRoomCount         int64
	PendingJoinRequestCount int64
}

func (s *RoomService) List(ctx context.Context, input ListRoomsInput) (*ListRoomsOutput, error) {
	return nil, fmt.Errorf("room service List not implemented")
}

func (s *RoomService) ListMineCreated(ctx context.Context, page, pageSize int) (*ListRoomsOutput, error) {
	return nil, fmt.Errorf("room service ListMineCreated not implemented")
}

func (s *RoomService) ListMineJoined(ctx context.Context, page, pageSize int) (*ListRoomsOutput, error) {
	return nil, fmt.Errorf("room service ListMineJoined not implemented")
}

func (s *RoomService) GetMyStats(ctx context.Context) (*UserStatsOutput, error) {
	return nil, fmt.Errorf("room service GetMyStats not implemented")
}

// GetByID returns the room and its members (with user info preloaded).
func (s *RoomService) GetByID(ctx context.Context, id uint) (*models.Room, []models.RoomMember, error) {
	room, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	members, err := s.repo.GetMembersByRoomID(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get members: %w", err)
	}
	return room, members, nil
}

// Create creates a new room and adds the current user as the owner member.
func (s *RoomService) Create(ctx context.Context, input CreateRoomInput) (*models.Room, error) {
	owner, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}

	// reservation_status tracks the TYYS booking state; seed it from need_reservation.
	reservationStatus := "not_required"
	if input.NeedReservation {
		reservationStatus = "pending"
	}

	inviteCode, err := generateInviteCode()
	if err != nil {
		return nil, fmt.Errorf("generate invite code: %w", err)
	}

	memberLimit := (*int)(nil)
	if input.MemberLimit != nil {
		v := int(*input.MemberLimit)
		memberLimit = &v
	}

	room := &models.Room{
		OwnerID:             owner.ID,
		Name:                input.Name,
		SportType:           input.SportType,
		CampusName:          input.CampusName,
		VenueName:           input.VenueName,
		Visibility:          input.Visibility,
		JoinMode:            input.JoinMode,
		Status:              "recruiting",
		ReservationStatus:   reservationStatus,
		ReservationProvider: "tyys",
		NeedReservation:     input.NeedReservation,
		StartTime:           input.StartTime,
		EndTime:             input.EndTime,
		MemberLimit:         memberLimit,
		InviteCode:          inviteCode,
	}
	if input.GenderRule != nil {
		room.GenderRule = *input.GenderRule
	}
	if input.Organization != nil {
		room.Organization = *input.Organization
	}
	if input.LevelDesc != nil {
		room.LevelDesc = *input.LevelDesc
	}
	if input.Description != nil {
		room.Description = *input.Description
	}

	if err := s.repo.Create(ctx, room); err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}

	// Add the creator as the first member with role "owner".
	now := time.Now()
	ownerMember := &models.RoomMember{
		RoomID:   room.ID,
		UserID:   owner.ID,
		Role:     "owner",
		Status:   "joined",
		JoinedAt: &now,
	}
	if err := s.repo.CreateMember(ctx, ownerMember); err != nil {
		return nil, fmt.Errorf("add owner as member: %w", err)
	}

	return room, nil
}

// generateInviteCode returns an 8-character random hex string used as the room invite code.
func generateInviteCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *RoomService) Update(ctx context.Context, roomID uint, input UpdateRoomInput) (*models.Room, error) {
	return nil, fmt.Errorf("room service Update not implemented")
}

func (s *RoomService) Close(ctx context.Context, roomID uint) (*models.Room, error) {
	return nil, fmt.Errorf("room service Close not implemented")
}

func (s *RoomService) JoinByCode(ctx context.Context, input JoinRoomByCodeInput) (*JoinRoomOutput, error) {
	return nil, fmt.Errorf("room service JoinByCode not implemented")
}

func (s *RoomService) JoinDirectly(ctx context.Context, roomID uint) (*JoinRoomOutput, error) {
	return nil, fmt.Errorf("room service JoinDirectly not implemented")
}

func (s *RoomService) CreateJoinRequest(ctx context.Context, roomID uint, input CreateJoinRequestInput) (*models.JoinRequest, error) {
	return nil, fmt.Errorf("room service CreateJoinRequest not implemented")
}

func (s *RoomService) ApproveJoinRequest(ctx context.Context, roomID uint, input ReviewJoinRequestInput) (*models.JoinRequest, error) {
	return nil, fmt.Errorf("room service ApproveJoinRequest not implemented")
}

func (s *RoomService) RejectJoinRequest(ctx context.Context, roomID uint, input ReviewJoinRequestInput) (*models.JoinRequest, error) {
	return nil, fmt.Errorf("room service RejectJoinRequest not implemented")
}

func (s *RoomService) InviteMember(ctx context.Context, roomID uint, input InviteMemberInput) (*models.RoomMember, error) {
	return nil, fmt.Errorf("room service InviteMember not implemented")
}

func (s *RoomService) RemoveMember(ctx context.Context, roomID, userID uint) (*models.RoomMember, error) {
	return nil, fmt.Errorf("room service RemoveMember not implemented")
}
