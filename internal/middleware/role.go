package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	RoleAdmin  = "admin"
	RoleViewer = "viewer"
	RoleUser   = "user"
	RoleGuest  = "guest"
)

func RequireRoles(allowed ...string) gin.HandlerFunc {
	set := make(map[string]bool, len(allowed))
	for _, r := range allowed {
		set[strings.ToLower(strings.TrimSpace(r))] = true
	}

	return func(c *gin.Context) {
		role := strings.ToLower(strings.TrimSpace(getRoleFromContext(c)))
		if role == "" || !set[role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Akses ditolak: role Anda tidak diizinkan untuk endpoint ini",
			})
			return
		}
		c.Next()
	}
}

func DenyRoles(denied ...string) gin.HandlerFunc {
	set := make(map[string]bool, len(denied))
	for _, r := range denied {
		set[strings.ToLower(strings.TrimSpace(r))] = true
	}

	return func(c *gin.Context) {
		role := strings.ToLower(strings.TrimSpace(getRoleFromContext(c)))
		if set[role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Akses ditolak: role guest hanya boleh mengakses VSS Monitoring",
			})
			return
		}
		c.Next()
	}
}

func getRoleFromContext(c *gin.Context) string {
	if v, ok := c.Get("role_name"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if v, ok := c.Get("role"); ok {
		switch t := v.(type) {
		case string:
			return t
		}
	}
	if v, ok := c.Get("user"); ok {
		if m, ok := v.(map[string]interface{}); ok {
			if rn, ok := m["role_name"].(string); ok {
				return rn
			}
			if rn, ok := m["role"].(string); ok {
				return rn
			}
		}
	}
	return ""
}