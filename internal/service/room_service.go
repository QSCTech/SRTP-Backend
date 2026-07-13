package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"
	"github.com/QSCTech/SRTP-Backend/pkg/utils"

	"gorm.io/gorm"
)

type RoomService struct {
	repo                *repository.RoomRepository
	userService         *UserService
	inviteCodeGenerator func() (string, error)
}

func NewRoomService(repo *repository.RoomRepository, userService *UserService) *RoomService {
	return &RoomService{repo: repo, userService: userService, inviteCodeGenerator: generateInviteCode}
}

const inviteCodeCreateAttempts = 5

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
	RequestPublicID string
}

type InviteMemberInput struct {
	UserPublicID string
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

func (s *RoomService) ListMineCreated(ctx context.Context, page, pageSize int) (*ListRoomsOutput, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize = normalizePagination(page, pageSize)
	result, err := s.repo.ListRoomsByOwner(ctx, currentUser.ID, page, pageSize)
	if err != nil {
		return nil, err
	}
	return s.buildRoomListOutput(ctx, result, page, pageSize)
}

func (s *RoomService) ListMineJoined(ctx context.Context, page, pageSize int) (*ListRoomsOutput, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize = normalizePagination(page, pageSize)
	result, err := s.repo.ListRoomsJoinedByUser(ctx, currentUser.ID, page, pageSize)
	if err != nil {
		return nil, err
	}
	return s.buildRoomListOutput(ctx, result, page, pageSize)
}

func (s *RoomService) GetMyStats(ctx context.Context) (*UserStatsOutput, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.CountRoomsByOwner(ctx, currentUser.ID)
	if err != nil {
		return nil, err
	}
	joined, err := s.repo.CountJoinedRoomsByUser(ctx, currentUser.ID)
	if err != nil {
		return nil, err
	}
	pending, err := s.repo.CountPendingJoinRequestsByUser(ctx, currentUser.ID)
	if err != nil {
		return nil, err
	}
	return &UserStatsOutput{
		CreatedRoomCount:        created,
		JoinedRoomCount:         joined,
		PendingJoinRequestCount: pending,
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
	if err := validateCreateRoomInput(input); err != nil {
		return nil, err
	}

	for attempt := 0; attempt < inviteCodeCreateAttempts; attempt++ {
		inviteCode, err := s.inviteCodeGenerator()
		if err != nil {
			return nil, fmt.Errorf("generate invite code: %w", err)
		}
		room := buildRoomForCreate(currentUser.ID, input, inviteCode)
		now := time.Now()
		owner := &models.RoomMember{
			UserID: currentUser.ID,
			Role:   "owner", Status: "joined", JoinedAt: &now,
		}
		if room.MemberLimit != nil && *room.MemberLimit == 1 {
			room.Status = "full"
		}
		if err := s.repo.CreateRoomWithOwner(ctx, room, owner); err == nil {
			room.Owner = *currentUser
			return room, nil
		} else if !s.inviteCodeExists(ctx, inviteCode) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("failed to generate a unique invite code after %d attempts", inviteCodeCreateAttempts)
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
	if err := applyRoomUpdate(room, input); err != nil {
		return nil, err
	}
	memberCount, err := s.repo.CountActiveMembers(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room.MemberLimit != nil {
		if int64(*room.MemberLimit) < memberCount {
			return nil, fmt.Errorf("member_limit cannot be less than current member count (%d)", memberCount)
		}
		if int64(*room.MemberLimit) == memberCount {
			room.Status = "full"
		} else {
			room.Status = "recruiting"
		}
	} else {
		room.Status = "recruiting"
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
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	code := strings.ToUpper(strings.TrimSpace(input.InviteCode))
	if code == "" {
		return nil, fmt.Errorf("invite_code is required")
	}
	room, err := s.repo.GetByInviteCode(ctx, code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("room not found")
		}
		return nil, err
	}
	return s.joinRoom(ctx, room.ID, currentUser, true)
}

func (s *RoomService) JoinDirectly(ctx context.Context, roomID uint) (*JoinRoomOutput, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	return s.joinRoom(ctx, roomID, currentUser, false)
}

func (s *RoomService) CreateJoinRequest(ctx context.Context, roomID uint, input CreateJoinRequestInput) (*models.JoinRequest, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	message := strings.TrimSpace(input.Message)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}
	if utf8.RuneCountInString(message) > 255 {
		return nil, fmt.Errorf("message must not exceed 255 characters")
	}
	var created *models.JoinRequest
	err = s.repo.Transaction(ctx, func(tx *repository.RoomRepository) error {
		room, err := tx.GetByIDForUpdate(ctx, roomID)
		if err != nil {
			return roomNotFoundError(err)
		}
		if room.OwnerID == currentUser.ID {
			return fmt.Errorf("room owner cannot apply to join")
		}
		if room.Visibility != "public" {
			return fmt.Errorf("private room can only be joined by invite code or owner invitation")
		}
		if room.JoinMode != "approval" {
			return fmt.Errorf("room does not require approval")
		}
		if err := ensureRoomRecruiting(room); err != nil {
			return err
		}
		if member, err := tx.GetMember(ctx, room.ID, currentUser.ID); err == nil && member.Status == "joined" {
			return fmt.Errorf("already a member")
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if _, err := tx.GetPendingJoinRequestByRoomAndUser(ctx, room.ID, currentUser.ID); err == nil {
			return fmt.Errorf("already have a pending join request")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := ensureCapacity(ctx, tx, room); err != nil {
			return err
		}
		created = &models.JoinRequest{RoomID: room.ID, UserID: currentUser.ID, Status: "pending", Message: message}
		return tx.CreateJoinRequest(ctx, created)
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *RoomService) CancelJoinRequest(ctx context.Context, roomID uint) (*models.JoinRequest, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	var cancelled *models.JoinRequest
	err = s.repo.Transaction(ctx, func(tx *repository.RoomRepository) error {
		if _, err := tx.GetByIDForUpdate(ctx, roomID); err != nil {
			return roomNotFoundError(err)
		}
		request, err := tx.GetPendingJoinRequestByRoomAndUser(ctx, roomID, currentUser.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("pending join request not found")
			}
			return err
		}
		request.Status = "cancelled"
		if err := tx.UpdateJoinRequest(ctx, request); err != nil {
			return err
		}
		cancelled = request
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cancelled, nil
}

func (s *RoomService) ApproveJoinRequest(ctx context.Context, roomID uint, input ReviewJoinRequestInput) (*models.JoinRequest, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	var approved *models.JoinRequest
	err = s.repo.Transaction(ctx, func(tx *repository.RoomRepository) error {
		room, err := tx.GetByIDForUpdate(ctx, roomID)
		if err != nil {
			return roomNotFoundError(err)
		}
		if room.OwnerID != currentUser.ID {
			return fmt.Errorf("only the owner can review join requests")
		}
		request, err := tx.GetJoinRequestByPublicIDForUpdate(ctx, input.RequestPublicID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("join request not found")
			}
			return err
		}
		if request.RoomID != room.ID {
			return fmt.Errorf("join request does not belong to this room")
		}
		if request.Status != "pending" {
			return fmt.Errorf("join request has already been reviewed")
		}
		if err := ensureRoomRecruiting(room); err != nil {
			return err
		}
		if err := ensureCapacity(ctx, tx, room); err != nil {
			return err
		}
		if _, err := joinMember(ctx, tx, room.ID, request.UserID, "member"); err != nil {
			return err
		}
		now := time.Now()
		request.Status, request.ReviewedBy, request.ReviewedAt = "approved", &currentUser.ID, &now
		if err := tx.UpdateJoinRequest(ctx, request); err != nil {
			return err
		}
		if err := updateRoomCapacityStatus(ctx, tx, room); err != nil {
			return err
		}
		approved = request
		return nil
	})
	if err != nil {
		return nil, err
	}
	return approved, nil
}

func (s *RoomService) RejectJoinRequest(ctx context.Context, roomID uint, input ReviewJoinRequestInput) (*models.JoinRequest, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	var rejected *models.JoinRequest
	err = s.repo.Transaction(ctx, func(tx *repository.RoomRepository) error {
		room, err := tx.GetByIDForUpdate(ctx, roomID)
		if err != nil {
			return roomNotFoundError(err)
		}
		if room.OwnerID != currentUser.ID {
			return fmt.Errorf("only the owner can review join requests")
		}
		request, err := tx.GetJoinRequestByPublicIDForUpdate(ctx, input.RequestPublicID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("join request not found")
			}
			return err
		}
		if request.RoomID != room.ID {
			return fmt.Errorf("join request does not belong to this room")
		}
		if request.Status != "pending" {
			return fmt.Errorf("join request has already been reviewed")
		}
		now := time.Now()
		request.Status, request.ReviewedBy, request.ReviewedAt = "rejected", &currentUser.ID, &now
		if err := tx.UpdateJoinRequest(ctx, request); err != nil {
			return err
		}
		rejected = request
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rejected, nil
}

func (s *RoomService) InviteMember(ctx context.Context, roomID uint, input InviteMemberInput) (*models.RoomMember, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	target, err := s.userService.GetByPublicID(ctx, input.UserPublicID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	var invited *models.RoomMember
	err = s.repo.Transaction(ctx, func(tx *repository.RoomRepository) error {
		room, err := tx.GetByIDForUpdate(ctx, roomID)
		if err != nil {
			return roomNotFoundError(err)
		}
		if room.OwnerID != currentUser.ID {
			return fmt.Errorf("only the owner can invite members")
		}
		if target.ID == currentUser.ID {
			return fmt.Errorf("cannot invite the room owner")
		}
		if err := ensureRoomRecruiting(room); err != nil {
			return err
		}
		if member, err := tx.GetMember(ctx, room.ID, target.ID); err == nil && member.Status == "joined" {
			return fmt.Errorf("user is already a member")
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := ensureCapacity(ctx, tx, room); err != nil {
			return err
		}
		invited, err = joinMember(ctx, tx, room.ID, target.ID, "member")
		if err != nil {
			return err
		}
		if err := approvePendingRequestIfPresent(ctx, tx, room.ID, target.ID, &currentUser.ID); err != nil {
			return err
		}
		return updateRoomCapacityStatus(ctx, tx, room)
	})
	if err != nil {
		return nil, err
	}
	invited.User = *target
	return invited, nil
}

func (s *RoomService) RemoveMember(ctx context.Context, roomID uint, userPublicID string) (*models.RoomMember, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	target, err := s.userService.GetByPublicID(ctx, userPublicID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	var removed *models.RoomMember
	err = s.repo.Transaction(ctx, func(tx *repository.RoomRepository) error {
		room, err := tx.GetByIDForUpdate(ctx, roomID)
		if err != nil {
			return roomNotFoundError(err)
		}
		if room.OwnerID != currentUser.ID {
			return fmt.Errorf("only the owner can remove members")
		}
		if room.Status != "recruiting" && room.Status != "full" {
			return fmt.Errorf("room is not active")
		}
		member, err := tx.GetMember(ctx, room.ID, target.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("user is not a member of this room")
			}
			return err
		}
		if member.Role == "owner" {
			return fmt.Errorf("cannot remove the room owner")
		}
		if member.Status != "joined" {
			return fmt.Errorf("user is not an active member of this room")
		}
		member.Status = "removed"
		if err := tx.UpdateMember(ctx, member); err != nil {
			return err
		}
		if err := updateRoomCapacityStatus(ctx, tx, room); err != nil {
			return err
		}
		removed = member
		return nil
	})
	if err != nil {
		return nil, err
	}
	removed.User = *target
	return removed, nil
}

func (s *RoomService) LeaveRoom(ctx context.Context, roomID uint) (*models.RoomMember, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	var left *models.RoomMember
	err = s.repo.Transaction(ctx, func(tx *repository.RoomRepository) error {
		room, err := tx.GetByIDForUpdate(ctx, roomID)
		if err != nil {
			return roomNotFoundError(err)
		}
		if room.OwnerID == currentUser.ID {
			return fmt.Errorf("room owner cannot leave; close the room instead")
		}
		if room.Status != "recruiting" && room.Status != "full" {
			return fmt.Errorf("room is not active")
		}
		member, err := tx.GetMember(ctx, room.ID, currentUser.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("not a member of this room")
			}
			return err
		}
		if member.Status != "joined" {
			return fmt.Errorf("not an active member of this room")
		}
		member.Status = "left"
		if err := tx.UpdateMember(ctx, member); err != nil {
			return err
		}
		if err := updateRoomCapacityStatus(ctx, tx, room); err != nil {
			return err
		}
		left = member
		return nil
	})
	if err != nil {
		return nil, err
	}
	left.User = *currentUser
	return left, nil
}

func (s *RoomService) ListJoinRequests(ctx context.Context, roomID uint, status *string) ([]models.JoinRequest, error) {
	currentUser, err := s.userService.GetCurrent(ctx)
	if err != nil {
		return nil, err
	}
	if status != nil {
		value := strings.TrimSpace(*status)
		switch value {
		case "pending", "approved", "rejected", "cancelled":
			status = &value
		default:
			return nil, fmt.Errorf("invalid join request status")
		}
	}
	room, err := s.repo.GetByID(ctx, roomID)
	if err != nil {
		return nil, roomNotFoundError(err)
	}
	if room.OwnerID != currentUser.ID {
		return nil, fmt.Errorf("only the owner can list join requests")
	}
	return s.repo.ListJoinRequests(ctx, room.ID, status)
}

func (s *RoomService) buildRoomListOutput(ctx context.Context, result *repository.RoomListResult, page, pageSize int) (*ListRoomsOutput, error) {
	roomIDs := make([]uint, 0, len(result.Items))
	for _, room := range result.Items {
		roomIDs = append(roomIDs, room.ID)
	}
	counts := make(map[uint]int64)
	if len(roomIDs) > 0 {
		var err error
		counts, err = s.repo.CountMembersByRoomIDs(ctx, roomIDs)
		if err != nil {
			return nil, err
		}
	}
	items := make([]RoomCardItem, 0, len(result.Items))
	for _, room := range result.Items {
		items = append(items, RoomCardItem{Room: room, CurrentMemberCount: int32(counts[room.ID])})
	}
	return &ListRoomsOutput{
		Page: int32(page), PageSize: int32(pageSize), Total: result.Total, Items: items,
	}, nil
}

func normalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func validateCreateRoomInput(input CreateRoomInput) error {
	name := utils.NormalizeWhitespace(input.Name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if utf8.RuneCountInString(name) > 32 {
		return fmt.Errorf("name must not exceed 32 characters")
	}
	if err := validateRequiredText("sport_type", input.SportType, 32); err != nil {
		return err
	}
	if err := validateRequiredText("campus_name", input.CampusName, 64); err != nil {
		return err
	}
	if err := validateRequiredText("venue_name", input.VenueName, 128); err != nil {
		return err
	}
	if !isValidVisibility(input.Visibility) {
		return fmt.Errorf("visibility must be 'public' or 'private'")
	}
	if !isValidJoinMode(input.JoinMode) {
		return fmt.Errorf("join_mode must be 'direct' or 'approval'")
	}
	if input.StartTime.IsZero() || input.EndTime.IsZero() {
		return fmt.Errorf("start_time and end_time are required")
	}
	if !input.EndTime.After(input.StartTime) {
		return fmt.Errorf("end_time must be after start_time")
	}
	if input.MemberLimit != nil && *input.MemberLimit < 1 {
		return fmt.Errorf("member_limit must be greater than 0")
	}
	if err := validateOptionalText("gender_rule", input.GenderRule, 32); err != nil {
		return err
	}
	if err := validateOptionalText("organization", input.Organization, 64); err != nil {
		return err
	}
	if err := validateOptionalText("level_desc", input.LevelDesc, 64); err != nil {
		return err
	}
	return validateOptionalText("description", input.Description, 500)
}

func buildRoomForCreate(ownerID uint, input CreateRoomInput, inviteCode string) *models.Room {
	room := &models.Room{
		OwnerID: ownerID, Name: utils.NormalizeWhitespace(input.Name),
		SportType: strings.TrimSpace(input.SportType), CampusName: utils.NormalizeWhitespace(input.CampusName),
		VenueName: utils.NormalizeWhitespace(input.VenueName), Visibility: strings.TrimSpace(input.Visibility),
		JoinMode: strings.TrimSpace(input.JoinMode), Status: "recruiting", ReservationProvider: "tyys",
		ReservationStatus: "not_required", NeedReservation: input.NeedReservation,
		StartTime: input.StartTime, EndTime: input.EndTime, InviteCode: inviteCode,
	}
	if input.NeedReservation {
		room.ReservationStatus = "pending"
	}
	if input.MemberLimit != nil {
		value := int(*input.MemberLimit)
		room.MemberLimit = &value
	}
	if input.GenderRule != nil {
		room.GenderRule = strings.TrimSpace(*input.GenderRule)
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
	return room
}

func applyRoomUpdate(room *models.Room, input UpdateRoomInput) error {
	if input.Name != nil {
		name := utils.NormalizeWhitespace(*input.Name)
		if name == "" {
			return fmt.Errorf("name is required")
		}
		if utf8.RuneCountInString(name) > 32 {
			return fmt.Errorf("name must not exceed 32 characters")
		}
		room.Name = name
	}
	if input.Visibility != nil {
		if !isValidVisibility(*input.Visibility) {
			return fmt.Errorf("visibility must be 'public' or 'private'")
		}
		room.Visibility = strings.TrimSpace(*input.Visibility)
	}
	if input.JoinMode != nil {
		if !isValidJoinMode(*input.JoinMode) {
			return fmt.Errorf("join_mode must be 'direct' or 'approval'")
		}
		room.JoinMode = strings.TrimSpace(*input.JoinMode)
	}
	if input.StartTime != nil {
		room.StartTime = *input.StartTime
	}
	if input.EndTime != nil {
		room.EndTime = *input.EndTime
	}
	if !room.EndTime.After(room.StartTime) {
		return fmt.Errorf("end_time must be after start_time")
	}
	if input.NeedReservation != nil {
		room.NeedReservation = *input.NeedReservation
		if *input.NeedReservation {
			if room.ReservationStatus == "not_required" {
				room.ReservationStatus = "pending"
			}
		} else {
			room.ReservationStatus = "not_required"
		}
	}
	if input.MemberLimit != nil {
		if *input.MemberLimit < 1 {
			return fmt.Errorf("member_limit must be greater than 0")
		}
		value := int(*input.MemberLimit)
		room.MemberLimit = &value
	}
	if err := validateOptionalText("gender_rule", input.GenderRule, 32); err != nil {
		return err
	}
	if input.GenderRule != nil {
		room.GenderRule = strings.TrimSpace(*input.GenderRule)
	}
	if err := validateOptionalText("organization", input.Organization, 64); err != nil {
		return err
	}
	if input.Organization != nil {
		room.Organization = utils.NormalizeWhitespace(*input.Organization)
	}
	if err := validateOptionalText("level_desc", input.LevelDesc, 64); err != nil {
		return err
	}
	if input.LevelDesc != nil {
		room.LevelDesc = strings.TrimSpace(*input.LevelDesc)
	}
	if err := validateOptionalText("description", input.Description, 500); err != nil {
		return err
	}
	if input.Description != nil {
		room.Description = strings.TrimSpace(*input.Description)
	}
	return nil
}

func validateRequiredText(field, value string, max int) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	if utf8.RuneCountInString(value) > max {
		return fmt.Errorf("%s must not exceed %d characters", field, max)
	}
	return nil
}

func validateOptionalText(field string, value *string, max int) error {
	if value != nil && utf8.RuneCountInString(strings.TrimSpace(*value)) > max {
		return fmt.Errorf("%s must not exceed %d characters", field, max)
	}
	return nil
}

func isValidVisibility(value string) bool {
	switch strings.TrimSpace(value) {
	case "public", "private":
		return true
	default:
		return false
	}
}

func isValidJoinMode(value string) bool {
	switch strings.TrimSpace(value) {
	case "direct", "approval":
		return true
	default:
		return false
	}
}

func (s *RoomService) inviteCodeExists(ctx context.Context, code string) bool {
	_, err := s.repo.GetByInviteCode(ctx, code)
	return err == nil
}

var inviteCodeAlphabet = []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")

func generateInviteCode() (string, error) {
	code := make([]byte, 8)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(inviteCodeAlphabet))))
		if err != nil {
			return "", err
		}
		code[i] = inviteCodeAlphabet[n.Int64()]
	}
	return string(code), nil
}

func (s *RoomService) joinRoom(ctx context.Context, roomID uint, currentUser *models.User, byInviteCode bool) (*JoinRoomOutput, error) {
	var output *JoinRoomOutput
	err := s.repo.Transaction(ctx, func(tx *repository.RoomRepository) error {
		room, err := tx.GetByIDForUpdate(ctx, roomID)
		if err != nil {
			return roomNotFoundError(err)
		}
		if room.OwnerID == currentUser.ID {
			return fmt.Errorf("room owner cannot join their own room")
		}
		if member, err := tx.GetMember(ctx, room.ID, currentUser.ID); err == nil && member.Status == "joined" {
			status := "joined"
			output = &JoinRoomOutput{RoomID: room.ID, RoomPublicID: room.PublicID, JoinResult: "already_joined", MemberStatus: &status}
			return nil
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := ensureRoomRecruiting(room); err != nil {
			return err
		}
		if !byInviteCode {
			if room.Visibility != "public" {
				return fmt.Errorf("private room can only be joined by invite code or owner invitation")
			}
			if room.JoinMode != "direct" {
				return fmt.Errorf("room requires approval")
			}
		}
		if err := ensureCapacity(ctx, tx, room); err != nil {
			return err
		}
		if _, err := joinMember(ctx, tx, room.ID, currentUser.ID, "member"); err != nil {
			return err
		}
		if byInviteCode {
			if err := approvePendingRequestIfPresent(ctx, tx, room.ID, currentUser.ID, nil); err != nil {
				return err
			}
		}
		if err := updateRoomCapacityStatus(ctx, tx, room); err != nil {
			return err
		}
		status := "joined"
		output = &JoinRoomOutput{RoomID: room.ID, RoomPublicID: room.PublicID, JoinResult: "joined", MemberStatus: &status}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}

func roomNotFoundError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("room not found")
	}
	return err
}

func ensureRoomRecruiting(room *models.Room) error {
	if room.Status != "recruiting" {
		if room.Status == "full" {
			return fmt.Errorf("room is full")
		}
		return fmt.Errorf("room is not recruiting")
	}
	return nil
}

func ensureCapacity(ctx context.Context, repo *repository.RoomRepository, room *models.Room) error {
	if room.MemberLimit == nil {
		return nil
	}
	count, err := repo.CountActiveMembers(ctx, room.ID)
	if err != nil {
		return err
	}
	if count >= int64(*room.MemberLimit) {
		return fmt.Errorf("room is full")
	}
	return nil
}

func joinMember(ctx context.Context, repo *repository.RoomRepository, roomID, userID uint, role string) (*models.RoomMember, error) {
	now := time.Now()
	member, err := repo.GetMember(ctx, roomID, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		member = &models.RoomMember{RoomID: roomID, UserID: userID, Role: role, Status: "joined", JoinedAt: &now}
		if err := repo.CreateMember(ctx, member); err != nil {
			return nil, err
		}
		return member, nil
	}
	if err != nil {
		return nil, err
	}
	member.Role, member.Status, member.JoinedAt = role, "joined", &now
	if err := repo.UpdateMember(ctx, member); err != nil {
		return nil, err
	}
	return member, nil
}

func approvePendingRequestIfPresent(ctx context.Context, repo *repository.RoomRepository, roomID, userID uint, reviewerID *uint) error {
	request, err := repo.GetPendingJoinRequestByRoomAndUser(ctx, roomID, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	now := time.Now()
	request.Status, request.ReviewedBy, request.ReviewedAt = "approved", reviewerID, &now
	return repo.UpdateJoinRequest(ctx, request)
}

func updateRoomCapacityStatus(ctx context.Context, repo *repository.RoomRepository, room *models.Room) error {
	status := "recruiting"
	if room.MemberLimit != nil {
		count, err := repo.CountActiveMembers(ctx, room.ID)
		if err != nil {
			return err
		}
		if count >= int64(*room.MemberLimit) {
			status = "full"
		}
	}
	if room.Status == status {
		return nil
	}
	room.Status = status
	return repo.Update(ctx, room)
}
