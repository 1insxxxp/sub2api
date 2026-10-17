package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/geoip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// GetIPGeo resolves IPs from the server's offline MMDB. Queried addresses remain
// in the authenticated POST body and are never forwarded to third parties.
func (h *UserHandler) GetIPGeo(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	middleware2.SkipAudit(c) // Read-only lookups must not persist the queried customer-IP list.
	if _, ok := middleware2.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ips, ok := bindIPGeoRequest(c)
	if !ok {
		return
	}
	lookup := h.ipGeoLookup
	if lookup == nil {
		lookup = geoip.Lookup
	}
	results, err := lookup(ips)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Offline IP geolocation is unavailable")
		return
	}
	response.Success(c, results)
}

func bindIPGeoRequest(c *gin.Context) ([]string, bool) {
	var req struct {
		IPs []string `json:"ips"`
	}
	if c.Request.ContentLength > 4096 {
		response.BadRequest(c, "Provide between 1 and 50 valid IP addresses")
		return nil, false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if err := c.ShouldBindJSON(&req); err != nil || geoip.ValidateIPs(req.IPs) != nil {
		response.BadRequest(c, "Provide between 1 and 50 valid IP addresses")
		return nil, false
	}
	return req.IPs, true
}
