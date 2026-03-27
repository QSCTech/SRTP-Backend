package api

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/serein6174/SRTP-backend/internal/api/gen"
	"github.com/serein6174/SRTP-backend/internal/middleware"
	"github.com/serein6174/SRTP-backend/internal/service"
	"go.uber.org/zap"
)

func NewRouter(log *zap.Logger, db *sql.DB, userService *service.UserService) *gin.Engine {
	engine := gin.New()
	engine.Use(middleware.Zap(log), middleware.Recovery(log))

	handler := NewHandler(db, userService)
	gen.RegisterHandlers(engine, handler)

	return engine
}
