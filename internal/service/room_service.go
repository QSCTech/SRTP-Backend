package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"
	"github.com/QSCTech/SRTP-Backend/pkg/utils"
	"gorm.io/gorm"
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
	SportCode    *string
	Organization *string
	TimeSlot     *string
	SkillLevel   *string
	Atmosphere   *string
	Page         int
	PageSize     int
}

type CreateRoomInput struct {
	Name              string
	SportCode         string
	Visibility        string
	JoinPolicy        string
	LocationText      string
	StartTime         time.Time
	EndTime           *time.Time
	GenderRequirement *string
	MinMembers        *int32
	MaxMembers        *int32
	OrganizationText  *string
	SkillLevel        *string
	Atmosphere        *string
	Description       *string
}

type JoinRoomByCodeInput struct {
	InviteCode string
}

type CreateJoinRequestInput struct {
	Message string
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

func (s *RoomService) List(ctx context.Context, input ListRoomsInput) (*ListRoomsOutput, error) {
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	result, err := s.repo.List(ctx, repository.RoomFilter{
		Keyword:      input.Keyword,
		SportCode:    input.SportCode,
		Organization: input.Organization,
		TimeSlot:     input.TimeSlot,
		SkillLevel:   input.SkillLevel,
		Atmosphere:   input.Atmosphere,
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		return nil, err
	}

	items := make([]RoomCardItem, 0, len(result.Items))
	for _, room := range result.Items {
		count, countErr := s.repo.CountActiveMembers(ctx, room.ID)
		if countErr != nil {
			return nil, countErr
		}
		items = append(items, RoomCardItem{
			Room:               room,
			CurrentMemberCount: int32(count),
		})
	}

	return &ListRoomsOutput{
		Page:     int32(page),
		PageSize: int32(pageSize),
		Total:    result.Total,
		Items:    items,
	}, nil
}

func (s *RoomService) GetByID(ctx context.Context, id uint) (*models.Room, []models.RoomMember, error) {
	room, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, fmt.Errorf("room not found")
		}
		return nil, nil, err
	}

	members, err := s.repo.GetMembersByRoomID(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	return room, members, nil
}

func (s *RoomService) Create(ctx context.Context, input CreateRoomInput) (*models.Room, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}

	name := utils.NormalizeWhitespace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(input.SportCode) == "" {
		return nil, fmt.Errorf("sport_code is required")
	}
	if strings.TrimSpace(input.Visibility) == "" {
		return nil, fmt.Errorf("visibility is required")
	}
	if strings.TrimSpace(input.JoinPolicy) == "" {
		return nil, fmt.Errorf("join_policy is required")
	}
	locationText := utils.NormalizeWhitespace(input.LocationText)
	if locationText == "" {
		return nil, fmt.Errorf("location_text is required")
	}
	if input.StartTime.IsZero() {
		return nil, fmt.Errorf("start_time is required")
	}

	room := &models.Room{
		OwnerID:      currentUser.ID,
		Name:         name,
		SportCode:    strings.TrimSpace(input.SportCode),
		Visibility:   strings.TrimSpace(input.Visibility),
		JoinPolicy:   strings.TrimSpace(input.JoinPolicy),
		LocationText: locationText,
		StartTime:    input.StartTime,
		EndTime:      input.EndTime,
		InviteCode:   generateInviteCode(),
	}
	if input.GenderRequirement != nil {
		room.GenderRequirement = strings.TrimSpace(*input.GenderRequirement)
	}
	if input.MinMembers != nil {
		value := int(*input.MinMembers)
		room.MinMembers = &value
	}
	if input.MaxMembers != nil {
		value := int(*input.MaxMembers)
		room.MaxMembers = &value
	}
	if input.OrganizationText != nil {
		room.OrganizationText = utils.NormalizeWhitespace(*input.OrganizationText)
	}
	if input.SkillLevel != nil {
		room.SkillLevel = strings.TrimSpace(*input.SkillLevel)
	}
	if input.Atmosphere != nil {
		room.Atmosphere = strings.TrimSpace(*input.Atmosphere)
	}
	if input.Description != nil {
		room.Description = strings.TrimSpace(*input.Description)
	}

	if err := s.repo.Create(ctx, room); err != nil {
		return nil, err
	}

	joinedStatus := "joined"
	joinedAt := time.Now()
	if err := s.repo.CreateMember(ctx, &models.RoomMember{
		RoomID:    room.ID,
		UserID:    currentUser.ID,
		Role:      "owner",
		Status:    joinedStatus,
		JoinedAt:  &joinedAt,
		CreatedAt: joinedAt,
		UpdatedAt: joinedAt,
	}); err != nil {
		return nil, err
	}

	room.Owner = *currentUser
	return room, nil
}

func (s *RoomService) JoinByCode(ctx context.Context, input JoinRoomByCodeInput) (*JoinRoomOutput, error) {
	inviteCode := strings.TrimSpace(input.InviteCode)
	if inviteCode == "" {
		return nil, fmt.Errorf("invite_code is required")
	}

	room, err := s.repo.GetByInviteCode(ctx, inviteCode)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("room not found")
		}
		return nil, err
	}

	return s.joinRoom(ctx, room)
}

func (s *RoomService) JoinDirectly(ctx context.Context, roomID uint) (*JoinRoomOutput, error) {
	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("room not found")
		}
		return nil, err
	}

	return s.joinRoom(ctx, room)
}

func (s *RoomService) CreateJoinRequest(ctx context.Context, roomID uint, input CreateJoinRequestInput) (*models.JoinRequest, error) {
	message := strings.TrimSpace(input.Message)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}

	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := s.repo.GetByID(ctx, roomID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("room not found")
		}
		return nil, err
	}

	if _, err := s.repo.GetMember(ctx, roomID, currentUser.ID); err == nil {
		return nil, fmt.Errorf("already joined")
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	req := &models.JoinRequest{
		RoomID:  roomID,
		UserID:  currentUser.ID,
		Status:  "pending",
		Message: message,
	}
	if err := s.repo.CreateJoinRequest(ctx, req); err != nil {
		return nil, err
	}

	return req, nil
}

func (s *RoomService) joinRoom(ctx context.Context, room *models.Room) (*JoinRoomOutput, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := s.repo.GetMember(ctx, room.ID, currentUser.ID); err == nil {
		status := "joined"
		return &JoinRoomOutput{RoomID: room.ID, JoinResult: "already_joined", MemberStatus: &status}, nil
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	if room.JoinPolicy == "approval_required" {
		pending := "pending"
		request := &models.JoinRequest{
			RoomID:  room.ID,
			UserID:  currentUser.ID,
			Status:  pending,
			Message: "joined by join endpoint",
		}
		if err := s.repo.CreateJoinRequest(ctx, request); err != nil {
			return nil, err
		}
		return &JoinRoomOutput{RoomID: room.ID, JoinResult: "request_created", RequestStatus: &pending}, nil
	}

	if room.MaxMembers != nil {
		count, err := s.repo.CountActiveMembers(ctx, room.ID)
		if err != nil {
			return nil, err
		}
		if count >= int64(*room.MaxMembers) {
			return nil, fmt.Errorf("room is full")
		}
	}

	joinedStatus := "joined"
	joinedAt := time.Now()
	if err := s.repo.CreateMember(ctx, &models.RoomMember{
		RoomID:    room.ID,
		UserID:    currentUser.ID,
		Role:      "member",
		Status:    joinedStatus,
		JoinedAt:  &joinedAt,
		CreatedAt: joinedAt,
		UpdatedAt: joinedAt,
	}); err != nil {
		return nil, err
	}

	return &JoinRoomOutput{RoomID: room.ID, JoinResult: "joined", MemberStatus: &joinedStatus}, nil
}

func generateInviteCode() string {
	return fmt.Sprintf("ROOM%06d", time.Now().UnixNano()%1000000)
}
