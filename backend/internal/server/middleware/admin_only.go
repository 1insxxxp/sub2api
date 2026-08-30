package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AdminOnly 管理员权限中间件
// 必须在JWTAuth中间件之后使用
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := GetUserRoleFromContext(c)
		if !ok {
			AbortWithError(c, 401, "UNAUTHORIZED", "User not found in context")
			return
		}

		// 检查是否为管理员
		if role != service.RoleAdmin {
			AbortWithError(c, 403, "FORBIDDEN", "Admin access required")
			return
		}

		c.Next()
	}
}

// ManagerPageOnly permits full admins and secondary admins to access the narrow
// manager-page API surface. It must be used after JWTAuth.
func ManagerPageOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := GetUserRoleFromContext(c)
		if !ok {
			AbortWithError(c, 401, "UNAUTHORIZED", "User not found in context")
			return
		}

		if role != service.RoleAdmin && role != service.RoleSubAdmin {
			AbortWithError(c, 403, "FORBIDDEN", "Manager page access required")
			return
		}

		c.Next()
	}
}
