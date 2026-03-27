package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serein6174/SRTP-backend/internal/api/gen"
	"github.com/serein6174/SRTP-backend/internal/service"
	"github.com/serein6174/SRTP-backend/pkg/response"
	"gorm.io/gorm"
)

type Handler struct {
	db          *sql.DB
	userService *service.UserService
}

func NewHandler(db *sql.DB, userService *service.UserService) *Handler {
	return &Handler{db: db, userService: userService}
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

	user, err := h.userService.Create(c.Request.Context(), req.Name)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(c, http.StatusCreated, gen.UserResponse{
		Id:        int64(user.ID),
		Name:      user.Name,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	})
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

	response.JSON(c, http.StatusOK, gen.UserResponse{
		Id:        int64(user.ID),
		Name:      user.Name,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	})
}
