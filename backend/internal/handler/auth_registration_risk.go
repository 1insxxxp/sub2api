package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RegistrationRiskContext carries source metadata through OAuth callbacks,
// pending-account completion, email registration, and final login ceremonies.
func (h *AuthHandler) RegistrationRiskContext(c *gin.Context) {
	path := c.FullPath()
	oauth := strings.Contains(path, "/auth/oauth/")
	if !oauth && !strings.HasSuffix(path, "/auth/registration-challenge") && !strings.HasSuffix(path, "/auth/register") && !strings.HasSuffix(path, "/auth/send-verify-code") && !strings.Contains(path, "/auth/login") && !strings.HasSuffix(path, "/passkey/login/finish") {
		c.Next()
		return
	}
	hash, _, err := checkinBrowserIdentity(c)
	if err != nil {
		response.InternalError(c, "Failed to initialize browser identity")
		c.Abort()
		return
	}
	ctx := service.WithRegistrationRiskSource(c.Request.Context(), &service.RegistrationRiskSource{
		IP: ip.GetSecurityClientIP(c, false), UserAgent: c.Request.UserAgent(), BrowserHash: hash, OAuth: oauth,
	})
	c.Request = c.Request.WithContext(ctx)
	c.Next()
	h.authService.FinishRegistrationRiskRequest(ctx)
}

func (h *AuthHandler) RegistrationChallenge(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	provider, err := h.authService.SignupChallengeProvider(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, service.ErrServiceUnavailable)
		return
	}
	response.Success(c, gin.H{"provider": provider})
}

// OAuth completion can reuse a previously verified start receipt. A proof on a
// retry is checked once and scoped to this browser and verified account email.
func (h *AuthHandler) verifyOAuthCompletionProof(c *gin.Context, email string, req oauthStartCaptchaRequest) bool {
	proof := captchaProof(req.TurnstileToken, req.TencentCaptchaTicket, req.TencentCaptchaRandstr)
	if proof == (service.CaptchaProof{}) {
		return true
	}
	if err := h.authService.VerifySignupCaptcha(c.Request.Context(), proof, ip.GetSecurityClientIP(c, false), email, false); err != nil {
		response.ErrorFrom(c, err)
		return false
	}
	return true
}
