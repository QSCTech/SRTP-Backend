package response

import "github.com/gin-gonic/gin"

type ErrorBody struct {
	Message string `json:"message"`
}

func JSON(c *gin.Context, status int, payload any) {
	c.JSON(status, payload)
}

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, ErrorBody{Message: message})
}
