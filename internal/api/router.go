package api

import (
	"context"
	"database/sql"

	"github.com/QSCTech/SRTP-Backend/internal/api/gen"
	"github.com/QSCTech/SRTP-Backend/internal/middleware"
	"github.com/QSCTech/SRTP-Backend/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(log *zap.Logger, db *sql.DB, userService *service.UserService, roomService *service.RoomService, reservationService *service.ReservationService) *gin.Engine {
	engine := gin.New()
	engine.Use(middleware.Zap(log), middleware.Recovery(log))
	engine.Use(func(c *gin.Context) {
		if mockUserID := c.GetHeader("X-Mock-User-ID"); mockUserID != "" {
			ctx := context.WithValue(c.Request.Context(), service.MockUserIDKey, mockUserID)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	})

	handler := NewHandler(db, userService, roomService, reservationService)
	gen.RegisterHandlers(engine, handler)

	return engine
}
