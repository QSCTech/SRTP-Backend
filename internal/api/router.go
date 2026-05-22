package api

import (
	"database/sql"

	"github.com/QSCTech/SRTP-Backend/internal/api/gen"
	"github.com/QSCTech/SRTP-Backend/internal/middleware"
	"github.com/QSCTech/SRTP-Backend/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(log *zap.Logger, db *sql.DB, userService *service.UserService, roomService *service.RoomService, reservationService *service.ReservationService, auditService *service.AuditService) *gin.Engine {
	engine := gin.New()
	engine.Use(middleware.Zap(log), middleware.Recovery(log))

	handler := NewHandler(db, userService, roomService, reservationService, auditService)
	gen.RegisterHandlers(engine, handler)

	return engine
}
