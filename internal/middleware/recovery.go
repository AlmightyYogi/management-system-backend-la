package middleware

import (
	"net/http"

	"github.com/AlmightyOggy/management-system/internal/utils"
	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func()  {
			if err := recover(); err != nil {
				utils.Error(c, http.StatusInternalServerError, "Internal Server Error", nil)
			}
		}()
		c.Next()
	}
}