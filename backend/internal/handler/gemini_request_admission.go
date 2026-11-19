package handler

import (
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Ingress already accounts for the decoded body. Keep headroom in the same
// instance-wide budget for JSON parsing, rewriting and the upstream payload
// while the request waits for a slot or retries on a different account.
func admitGeminiRequestBody(c *gin.Context, size int, inference bool) bool {
	const conversionAllowance = 3
	err := pkghttputil.ReserveRequestBodyMemory(c.Request.Context(), int64(size)*conversionAllowance)
	if middleware.RejectRequestBodyMemoryOverload(c, err) {
		return false
	}
	if inference && size >= 4<<20 {
		service.BeginGeminiLargeRequestBudget(c)
	}
	return true
}
