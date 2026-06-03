package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"
	"github.com/QSCTech/SRTP-Backend/pkg/utils"

	"errors"
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

// =============================================================================
// Part 5 (5组) - 房间创建与房间管理：Create, Update, Close
// Part 6 (6组) - 成员加入与房主审批(临时补充实现，待6组替换): JoinByCode, JoinDirectly,
//               CreateJoinRequest, ApproveJoinRequest, RejectJoinRequest,
//               InviteMember, RemoveMember, joinRoom
// Part 3 (3组) - 登录与用户资料(临时补充实现，待3组替换): ListMineCreated,
//               ListMineJoined, GetMyStats
// =============================================================================

type JoinRoomByCodeInput struct {
	BuddyCode string
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
	RoomPublicID  string
	JoinResult    string
	MemberStatus  *string
	RequestStatus *string
}

type UserStatsOutput struct {
	CreatedRoomCount        int64
	JoinedRoomCount         int64
	PendingJoinRequestCount int64
}

/*基础功能：拿数据、加工成RoomCardItem*/
func (s *RoomService) List(ctx context.Context, input ListRoomsInput) (*ListRoomsOutput, error) {
	filter := repository.RoomFilter{
		Keyword:      input.Keyword,
		SportType:    input.SportType,
		Campus:       input.Campus,
		Date:         input.Date,
		TimeRange:    input.TimeRange,
		Organization: input.Organization,
		Level:        input.Level,
		Page:         int(input.Page),
		PageSize:     int(input.PageSize),
	}

	result, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	rooms := result.Items

	roomIDs := make([]uint, 0, len(rooms))
	for _, r := range rooms {
		roomIDs = append(roomIDs, r.ID)
	}

	countMap := make(map[uint]int64)
	if len(roomIDs) > 0 {
		countMap, err = s.repo.CountMembersByRoomIDs(ctx, roomIDs)
		if err != nil {
			return nil, err
		}
	}

	items := make([]RoomCardItem, 0, len(rooms))
	for _, r := range rooms {
		count := countMap[r.ID]

		items = append(items, RoomCardItem{
			Room:               r,
			CurrentMemberCount: int32(count),
		})
	}

	return &ListRoomsOutput{
		Page:     int32(input.Page),
		PageSize: int32(input.PageSize),
		Total:    result.Total,
		Items:    items,
	}, nil
}

/*基础功能：拿数据、判断*/

func (s *RoomService) GetByPublicID(ctx context.Context, publicID string) (*models.Room, []models.RoomMember, error) {
    room, err := s.repo.GetByPublicID(ctx, publicID)
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            var ErrRoomNotFound = errors.New("room not found")
            return nil, nil, ErrRoomNotFound
        }
        return nil, nil, err
    }

    members, err := s.repo.GetMembersByRoomID(ctx, room.ID)
    if err != nil {
        return nil, nil, err
    }

    return room, members, nil
}

func (s *RoomService) GetByID(ctx context.Context, id uint) (*models.Room, []models.RoomMember, error) {
    room, err := s.repo.GetByID(ctx, id)
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            var ErrRoomNotFound = errors.New("room not found")
			return nil, nil, ErrRoomNotFound
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
	if strings.TrimSpace(input.SportType) == "" {
		return nil, fmt.Errorf("sport_type is required")
	}
	if strings.TrimSpace(input.CampusName) == "" {
		return nil, fmt.Errorf("campus_name is required")
	}
	if strings.TrimSpace(input.VenueName) == "" {
		return nil, fmt.Errorf("venue_name is required")
	}
	if strings.TrimSpace(input.Visibility) == "" {
		return nil, fmt.Errorf("visibility is required")
	}
	if strings.TrimSpace(input.JoinMode) == "" {
		return nil, fmt.Errorf("join_mode is required")
	}
	if !isValidVisibility(input.Visibility) {
		return nil, fmt.Errorf("visibility must be 'public' or 'private'")
	}
	if !isValidJoinMode(input.JoinMode) {
		return nil, fmt.Errorf("join_mode must be 'direct', 'approval', or 'invite_only'")
	}
	if input.StartTime.IsZero() {
		return nil, fmt.Errorf("start_time is required")
	}
	if input.EndTime.IsZero() {
		return nil, fmt.Errorf("end_time is required")
	}

	if !input.EndTime.After(input.StartTime) {
		return nil, fmt.Errorf("end_time must be after start_time")
	}

	if isBuddyCodeSport(input.SportType) {
		if input.MemberLimit == nil || *input.MemberLimit < 2 {
			return nil, fmt.Errorf("buddy-code sport requires at least 2 members")
		}
	}

	room := &models.Room{
		OwnerID:          currentUser.ID,
		Name:             name,
		SportType:        strings.TrimSpace(input.SportType),
		CampusName:       utils.NormalizeWhitespace(input.CampusName),
		VenueName:        utils.NormalizeWhitespace(input.VenueName),
		Visibility:       strings.TrimSpace(input.Visibility),
		JoinMode:         strings.TrimSpace(input.JoinMode),
		StartTime:        input.StartTime,
		EndTime:          input.EndTime,
		NeedReservation:  input.NeedReservation,
		InviteCode:       generateInviteCode(),
	}
	if input.GenderRule != nil {
		room.GenderRule = strings.TrimSpace(*input.GenderRule)
	}
	if input.MemberLimit != nil {
		value := int(*input.MemberLimit)
		room.MemberLimit = &value
	}
	if input.Organization != nil {
		room.Organization = utils.NormalizeWhitespace(*input.Organization)
	}
	if input.LevelDesc != nil {
		room.LevelDesc = strings.TrimSpace(*input.LevelDesc)
	}
	if input.Description != nil {
		room.Description = strings.TrimSpace(*input.Description)
	}

	if room.NeedReservation {
		room.ReservationStatus = "pending"
	}

	if room.MemberLimit != nil && *room.MemberLimit <= 0 {
		return nil, fmt.Errorf("member_limit must be greater than 0")
	}

	now := time.Now()
	if err := s.repo.CreateRoomWithOwner(ctx, room, &models.RoomMember{
		UserID:    currentUser.ID,
		Role:      "owner",
		Status:    "joined",
		JoinedAt:  &now,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return nil, err
	}
	tryMarkFull(ctx, s.repo, room)

	room.Owner = *currentUser
	return room, nil
}

func (s *RoomService) Update(ctx context.Context, roomID uint, input UpdateRoomInput) (*models.Room, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}

	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("room not found")
		}
		return nil, err
	}

	if room.OwnerID != currentUser.ID {
		return nil, fmt.Errorf("only the owner can update the room")
	}

	if room.Status != "recruiting" && room.Status != "full" {
		return nil, fmt.Errorf("room is not active")
	}

	if input.Name != nil {
		name := utils.NormalizeWhitespace(*input.Name)
		if name == "" {
			return nil, fmt.Errorf("name is required")
		}
		room.Name = name
	}
	if input.Visibility != nil {
		if !isValidVisibility(*input.Visibility) {
			return nil, fmt.Errorf("visibility must be 'public' or 'private'")
		}
		room.Visibility = strings.TrimSpace(*input.Visibility)
	}
	if input.JoinMode != nil {
		if !isValidJoinMode(*input.JoinMode) {
			return nil, fmt.Errorf("join_mode must be 'direct', 'approval', or 'invite_only'")
		}
		room.JoinMode = strings.TrimSpace(*input.JoinMode)
	}
	if input.StartTime != nil {
		room.StartTime = *input.StartTime
	}
	if input.EndTime != nil {
		room.EndTime = *input.EndTime
	}
	if input.NeedReservation != nil {
		room.NeedReservation = *input.NeedReservation
		if *input.NeedReservation && room.ReservationStatus == "not_required" {
			room.ReservationStatus = "pending"
		} else if !*input.NeedReservation {
			room.ReservationStatus = "not_required"
		}
	}
	if input.GenderRule != nil {
		room.GenderRule = strings.TrimSpace(*input.GenderRule)
	}
	if input.MemberLimit != nil {
		value := int(*input.MemberLimit)
		if value <= 0 {
			return nil, fmt.Errorf("member_limit must be greater than 0")
		}
		count, err := s.repo.CountActiveMembers(ctx, roomID)
		if err != nil {
			return nil, err
		}
		if value < int(count) {
			return nil, fmt.Errorf("member_limit cannot be less than current member count (%d)", count)
		}
		room.MemberLimit = &value
		// 扩大容量时若房间已满，恢复为招募中
		if room.Status == "full" && value > int(count) {
			room.Status = "recruiting"
		}
	}
	if input.Organization != nil {
		room.Organization = utils.NormalizeWhitespace(*input.Organization)
	}
	if input.LevelDesc != nil {
		room.LevelDesc = strings.TrimSpace(*input.LevelDesc)
	}
	if input.Description != nil {
		room.Description = strings.TrimSpace(*input.Description)
	}

	if !room.EndTime.After(room.StartTime) {
		return nil, fmt.Errorf("end_time must be after start_time")
	}

	if err := s.repo.Update(ctx, room); err != nil {
		return nil, err
	}

	return room, nil
}

func (s *RoomService) Close(ctx context.Context, roomID uint) (*models.Room, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}

	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("room not found")
		}
		return nil, err
	}

	if room.OwnerID != currentUser.ID {
		return nil, fmt.Errorf("only the owner can close the room")
	}

	if room.Status == "cancelled" {
		return nil, fmt.Errorf("room is already cancelled")
	}

	room.Status = "cancelled"
	if err := s.repo.Update(ctx, room); err != nil {
		return nil, err
	}

	return room, nil
}

func (s *RoomService) JoinByCode(ctx context.Context, input JoinRoomByCodeInput) (*JoinRoomOutput, error) {
	buddyCode := strings.TrimSpace(input.BuddyCode)
	if buddyCode == "" {
		return nil, fmt.Errorf("buddy_code is required")
	}

	room, err := s.repo.GetByBuddyCode(ctx, buddyCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("room not found")
		}
		return nil, err
	}

	return s.joinRoom(ctx, room, true)
}

func (s *RoomService) JoinDirectly(ctx context.Context, roomID uint) (*JoinRoomOutput, error) {
	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("room not found")
		}
		return nil, err
	}

	return s.joinRoom(ctx, room, false)
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("room not found")
		}
		return nil, err
	}

	if _, err := s.repo.GetMember(ctx, roomID, currentUser.ID); err == nil {
		return nil, fmt.Errorf("already a member")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if _, err := s.repo.GetPendingJoinRequestByRoomAndUser(ctx, roomID, currentUser.ID); err == nil {
		return nil, fmt.Errorf("already have a pending join request")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
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

func (s *RoomService) ApproveJoinRequest(ctx context.Context, roomID uint, input ReviewJoinRequestInput) (*models.JoinRequest, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}

	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("room not found")
		}
		return nil, err
	}

	if room.OwnerID != currentUser.ID {
		return nil, fmt.Errorf("only the owner can review join requests")
	}

	req, err := s.repo.GetJoinRequestByID(ctx, input.RequestID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("join request not found")
		}
		return nil, err
	}

	if req.RoomID != roomID {
		return nil, fmt.Errorf("join request does not belong to this room")
	}

	if req.Status != "pending" {
		return nil, fmt.Errorf("join request has already been reviewed")
	}

	if room.Status != "recruiting" {
		return nil, fmt.Errorf("room is not recruiting")
	}

	if _, err := s.repo.GetMember(ctx, roomID, req.UserID); err == nil {
		return nil, fmt.Errorf("user is already a member")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if room.MemberLimit != nil {
		count, err := s.repo.CountActiveMembers(ctx, roomID)
		if err != nil {
			return nil, err
		}
		if count >= int64(*room.MemberLimit) {
			return nil, fmt.Errorf("room is full")
		}
	}

	now := time.Now()
	req.Status = "approved"
	req.ReviewedBy = &currentUser.ID
	req.ReviewedAt = &now

	if err := s.repo.CreateMember(ctx, &models.RoomMember{
		RoomID:    roomID,
		UserID:    req.UserID,
		Role:      "member",
		Status:    "joined",
		JoinedAt:  &now,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return nil, err
	}
	tryMarkFull(ctx, s.repo, room)

	if err := s.repo.UpdateJoinRequest(ctx, req); err != nil {
		return nil, err
	}

	return req, nil
}

func (s *RoomService) RejectJoinRequest(ctx context.Context, roomID uint, input ReviewJoinRequestInput) (*models.JoinRequest, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}

	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("room not found")
		}
		return nil, err
	}

	if room.OwnerID != currentUser.ID {
		return nil, fmt.Errorf("only the owner can review join requests")
	}

	req, err := s.repo.GetJoinRequestByID(ctx, input.RequestID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("join request not found")
		}
		return nil, err
	}

	if req.RoomID != roomID {
		return nil, fmt.Errorf("join request does not belong to this room")
	}

	if req.Status != "pending" {
		return nil, fmt.Errorf("join request has already been reviewed")
	}

	now := time.Now()
	req.Status = "rejected"
	req.ReviewedBy = &currentUser.ID
	req.ReviewedAt = &now

	if err := s.repo.UpdateJoinRequest(ctx, req); err != nil {
		return nil, err
	}

	return req, nil
}

func (s *RoomService) InviteMember(ctx context.Context, roomID uint, input InviteMemberInput) (*models.RoomMember, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}

	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("room not found")
		}
		return nil, err
	}

	if room.OwnerID != currentUser.ID {
		return nil, fmt.Errorf("only the owner can invite members")
	}

	if room.Status != "recruiting" {
		return nil, fmt.Errorf("room is not recruiting")
	}

	if input.UserID == currentUser.ID {
		return nil, fmt.Errorf("cannot invite yourself")
	}

	targetUser, err := s.userService.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	if _, err := s.repo.GetMember(ctx, roomID, input.UserID); err == nil {
		return nil, fmt.Errorf("user is already a member")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if room.MemberLimit != nil {
		count, err := s.repo.CountActiveMembers(ctx, roomID)
		if err != nil {
			return nil, err
		}
		if count >= int64(*room.MemberLimit) {
			return nil, fmt.Errorf("room is full")
		}
	}

	now := time.Now()
	member := &models.RoomMember{
		RoomID:    roomID,
		UserID:    input.UserID,
		Role:      "member",
		Status:    "joined",
		JoinedAt:  &now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreateMember(ctx, member); err != nil {
		return nil, err
	}
	tryMarkFull(ctx, s.repo, room)

	member.User = *targetUser
	return member, nil
}

func (s *RoomService) RemoveMember(ctx context.Context, roomID, userID uint) (*models.RoomMember, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}

	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("room not found")
		}
		return nil, err
	}

	if room.OwnerID != currentUser.ID {
		return nil, fmt.Errorf("only the owner can remove members")
	}

	if room.Status != "recruiting" {
		return nil, fmt.Errorf("room is not recruiting")
	}

	member, err := s.repo.GetMember(ctx, roomID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user is not a member of this room")
		}
		return nil, err
	}

	if member.Role == "owner" {
		return nil, fmt.Errorf("cannot remove the room owner")
	}

	if err := s.repo.DeleteMember(ctx, roomID, userID); err != nil {
		return nil, err
	}

	return member, nil
}

func (s *RoomService) ListMineCreated(ctx context.Context, page, pageSize int) (*ListRoomsOutput, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	result, err := s.repo.ListRoomsByOwner(ctx, currentUser.ID, page, pageSize)
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

func (s *RoomService) ListMineJoined(ctx context.Context, page, pageSize int) (*ListRoomsOutput, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	result, err := s.repo.ListRoomsJoinedByUser(ctx, currentUser.ID, page, pageSize)
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

func (s *RoomService) GetMyStats(ctx context.Context) (*UserStatsOutput, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}

	createdCount, err := s.repo.CountRoomsByOwner(ctx, currentUser.ID)
	if err != nil {
		return nil, err
	}

	joinedCount, err := s.repo.CountJoinedRoomsByUser(ctx, currentUser.ID)
	if err != nil {
		return nil, err
	}

	pendingCount, err := s.repo.CountPendingJoinRequestsByUser(ctx, currentUser.ID)
	if err != nil {
		return nil, err
	}

	return &UserStatsOutput{
		CreatedRoomCount:        createdCount,
		JoinedRoomCount:         joinedCount,
		PendingJoinRequestCount: pendingCount,
	}, nil
}

func (s *RoomService) joinRoom(ctx context.Context, room *models.Room, bypassJoinMode bool) (*JoinRoomOutput, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := s.repo.GetMember(ctx, room.ID, currentUser.ID); err == nil {
		status := "joined"
		return &JoinRoomOutput{RoomID: room.ID, RoomPublicID: room.PublicID, JoinResult: "already_joined", MemberStatus: &status}, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if room.Status != "recruiting" {
		return nil, fmt.Errorf("room is not recruiting")
	}

	if room.JoinMode == "invite_only" && !bypassJoinMode {
		return nil, fmt.Errorf("room is invite-only, please use invite code")
	}

	if room.JoinMode == "approval" {
		if _, err := s.repo.GetPendingJoinRequestByRoomAndUser(ctx, room.ID, currentUser.ID); err == nil {
			return nil, fmt.Errorf("already have a pending join request")
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		pending := "pending"
		request := &models.JoinRequest{
			RoomID:  room.ID,
			UserID:  currentUser.ID,
			Status:  pending,
			Message: "joined via direct join",
		}
		if err := s.repo.CreateJoinRequest(ctx, request); err != nil {
			return nil, err
		}
		return &JoinRoomOutput{RoomID: room.ID, RoomPublicID: room.PublicID, JoinResult: "request_created", RequestStatus: &pending}, nil
	}

	if room.MemberLimit != nil {
		count, err := s.repo.CountActiveMembers(ctx, room.ID)
		if err != nil {
			return nil, err
		}
		if count >= int64(*room.MemberLimit) {
			return nil, fmt.Errorf("room is full")
		}
	}

	now := time.Now()
	if err := s.repo.CreateMember(ctx, &models.RoomMember{
		RoomID:    room.ID,
		UserID:    currentUser.ID,
		Role:      "member",
		Status:    "joined",
		JoinedAt:  &now,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return nil, err
	}

	joinedStatus := "joined"
	tryMarkFull(ctx, s.repo, room)
	return &JoinRoomOutput{RoomID: room.ID, RoomPublicID: room.PublicID, JoinResult: "joined", MemberStatus: &joinedStatus}, nil
}

var inviteCodeChars = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func generateInviteCode() string {
	code := make([]byte, 8)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(inviteCodeChars))))
		code[i] = inviteCodeChars[n.Int64()]
	}
	return string(code)
}

func tryMarkFull(ctx context.Context, repo *repository.RoomRepository, room *models.Room) {
	if room.MemberLimit == nil {
		return
	}
	count, err := repo.CountActiveMembers(ctx, room.ID)
	if err != nil {
		return
	}
	if count >= int64(*room.MemberLimit) && room.Status == "recruiting" {
		room.Status = "full"
		_ = repo.Update(ctx, room)
	}
}

func isValidVisibility(v string) bool {
	switch strings.TrimSpace(v) {
	case "public", "private":
		return true
	default:
		return false
	}
}

func isValidJoinMode(m string) bool {
	switch strings.TrimSpace(m) {
	case "direct", "approval", "invite_only":
		return true
	default:
		return false
	}
}

func isBuddyCodeSport(sportType string) bool {
	switch strings.ToLower(strings.TrimSpace(sportType)) {
	case "tennis", "badminton":
		return true
	default:
		return false
	}
}
