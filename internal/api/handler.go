package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/QSCTech/SRTP-Backend/internal/api/gen"
	"github.com/QSCTech/SRTP-Backend/internal/service"
	"github.com/QSCTech/SRTP-Backend/models"
	"github.com/QSCTech/SRTP-Backend/pkg/response"
	"gorm.io/gorm"
)

type Handler struct {
	db          *sql.DB
	userService *service.UserService
	roomService *service.RoomService
}

func NewHandler(db *sql.DB, userService *service.UserService, roomService *service.RoomService) *Handler {
	return &Handler{db: db, userService: userService, roomService: roomService}
}

func (h *Handler) GetHealthz(c *gin.Context) {
	response.JSON(c, http.StatusOK, gen.HealthResponse{
		Service: "srtp-backend",
		Status:  "ok",
	})
}

func (h *Handler) GetReadyz(c *gin.Context) {
	if h.db == nil {
		response.Error(c, http.StatusServiceUnavailable, "database unavailable")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		response.Error(c, http.StatusServiceUnavailable, "database down")
		return
	}

	response.JSON(c, http.StatusOK, gen.ReadyResponse{
		Database: "up",
		Status:   "ready",
	})
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req gen.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.userService.Create(c.Request.Context(), req.AuthUid)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(c, http.StatusCreated, buildUserResponse(user))
}

func (h *Handler) GetUserById(c *gin.Context, id int64) {
	user, err := h.userService.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || err.Error() == "user not found" {
			response.Error(c, http.StatusNotFound, "user not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "failed to fetch user")
		return
	}

	response.JSON(c, http.StatusOK, buildUserResponse(user))
}

func (h *Handler) ListRooms(c *gin.Context, params gen.ListRoomsParams) {
	rooms, err := h.roomService.List(c.Request.Context(), service.ListRoomsInput{
		Keyword:      params.Keyword,
		SportCode:    params.SportCode,
		Organization: params.Organization,
		TimeSlot:     params.TimeSlot,
		SkillLevel:   params.SkillLevel,
		Atmosphere:   params.Atmosphere,
		Page:         optionalInt32(params.Page, 1),
		PageSize:     optionalInt32(params.PageSize, 20),
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to list rooms")
		return
	}

	items := make([]gen.RoomCard, 0, len(rooms.Items))
	for _, item := range rooms.Items {
		items = append(items, buildRoomCard(item))
	}

	response.JSON(c, http.StatusOK, gen.RoomCardPage{
		Page:     rooms.Page,
		PageSize: rooms.PageSize,
		Total:    rooms.Total,
		Items:    items,
	})
}

func (h *Handler) CreateRoom(c *gin.Context) {
	var req gen.CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	room, err := h.roomService.Create(c.Request.Context(), service.CreateRoomInput{
		Name:              req.Name,
		SportCode:         req.SportCode,
		Visibility:        req.Visibility,
		JoinPolicy:        req.JoinPolicy,
		LocationText:      req.LocationText,
		StartTime:         req.StartTime,
		EndTime:           req.EndTime,
		GenderRequirement: req.GenderRequirement,
		MinMembers:        req.MinMembers,
		MaxMembers:        req.MaxMembers,
		OrganizationText:  req.OrganizationText,
		SkillLevel:        req.SkillLevel,
		Atmosphere:        req.Atmosphere,
		Description:       req.Description,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	room, members, memberErr := h.roomService.GetByID(c.Request.Context(), room.ID)
	if memberErr != nil {
		response.Error(c, http.StatusInternalServerError, "failed to fetch room")
		return
	}

	response.JSON(c, http.StatusCreated, buildRoomDetail(room, members))
}

func (h *Handler) GetRoomById(c *gin.Context, roomId int64) {
	room, members, err := h.roomService.GetByID(c.Request.Context(), uint(roomId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || err.Error() == "room not found" {
			response.Error(c, http.StatusNotFound, "room not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "failed to fetch room")
		return
	}

	response.JSON(c, http.StatusOK, buildRoomDetail(room, members))
}

func (h *Handler) JoinRoomByCode(c *gin.Context) {
	var req gen.JoinRoomByCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.roomService.JoinByCode(c.Request.Context(), service.JoinRoomByCodeInput{InviteCode: req.InviteCode})
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(c, http.StatusOK, buildJoinRoomResult(result))
}

func (h *Handler) JoinRoomDirectly(c *gin.Context, roomId int64) {
	result, err := h.roomService.JoinDirectly(c.Request.Context(), uint(roomId))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(c, http.StatusOK, buildJoinRoomResult(result))
}

func (h *Handler) CreateJoinRequest(c *gin.Context, roomId int64) {
	var req gen.CreateJoinRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	joinRequest, err := h.roomService.CreateJoinRequest(c.Request.Context(), uint(roomId), service.CreateJoinRequestInput{Message: req.Message})
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(c, http.StatusCreated, gen.JoinRequestResponse{
		RequestId: int64(joinRequest.ID),
		Status:    joinRequest.Status,
	})
}

func buildUserResponse(user *models.User) gen.UserResponse {
	return gen.UserResponse{
		Id:            int64(user.ID),
		AuthUid:       user.AuthUID,
		Nickname:      user.Nickname,
		AvatarUrl:     user.AvatarURL,
		Gender:        user.Gender,
		Bio:           user.Bio,
		ProfileStatus: user.ProfileStatus,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}
}

func buildRoomCard(item service.RoomCardItem) gen.RoomCard {
	room := item.Room
	return gen.RoomCard{
		Id:                 int64(room.ID),
		Name:               room.Name,
		SportCode:          room.SportCode,
		StartTime:          room.StartTime,
		LocationText:       room.LocationText,
		OwnerNickname:      room.Owner.Nickname,
		OwnerAvatarUrl:     room.Owner.AvatarURL,
		CurrentMemberCount: item.CurrentMemberCount,
		MaxMemberCount:     int32Value(room.MaxMembers),
		JoinPolicy:         room.JoinPolicy,
		Status:             room.Status,
	}
}

func buildRoomDetail(room *models.Room, members []models.RoomMember) gen.RoomDetail {
	memberItems := make([]gen.RoomMember, 0, len(members))
	for _, member := range members {
		memberItems = append(memberItems, gen.RoomMember{
			UserId:    int64(member.UserID),
			Nickname:  member.User.Nickname,
			AvatarUrl: member.User.AvatarURL,
			Role:      member.Role,
			Status:    member.Status,
		})
	}

	return gen.RoomDetail{
		Id:                 int64(room.ID),
		Name:               room.Name,
		SportCode:          room.SportCode,
		Visibility:         room.Visibility,
		JoinPolicy:         room.JoinPolicy,
		Status:             room.Status,
		LocationText:       room.LocationText,
		StartTime:          room.StartTime,
		EndTime:            room.EndTime,
		GenderRequirement:  room.GenderRequirement,
		MinMembers:         optionalInt(room.MinMembers),
		MaxMembers:         optionalInt(room.MaxMembers),
		OrganizationText:   room.OrganizationText,
		SkillLevel:         room.SkillLevel,
		Atmosphere:         room.Atmosphere,
		Description:        room.Description,
		Owner:              gen.RoomOwner{Id: int64(room.Owner.ID), Nickname: room.Owner.Nickname, AvatarUrl: room.Owner.AvatarURL},
		Members:            memberItems,
		CurrentMemberCount: int32(countCurrentMembers(members)),
		IsOwner:            false,
		Joinable:           room.Status == "open",
	}
}

func buildJoinRoomResult(result *service.JoinRoomOutput) gen.JoinRoomResult {
	return gen.JoinRoomResult{
		RoomId:        int64(result.RoomID),
		JoinResult:    result.JoinResult,
		MemberStatus:  result.MemberStatus,
		RequestStatus: result.RequestStatus,
	}
}

func optionalInt(value *int) *int32 {
	if value == nil {
		return nil
	}
	converted := int32(*value)
	return &converted
}

func int32Value(value *int) int32 {
	if value == nil {
		return 0
	}
	return int32(*value)
}

func optionalInt32(value *int32, fallback int) int {
	if value == nil {
		return fallback
	}
	return int(*value)
}

func countCurrentMembers(members []models.RoomMember) int32 {
	var count int32
	for _, member := range members {
		if member.Status == "joined" {
			count++
		}
	}
	return count
}
