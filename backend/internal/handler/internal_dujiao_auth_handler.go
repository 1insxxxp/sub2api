package handler

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const internalDujiaoSecretHeader = "X-Sub2-Internal-Secret"

type DujiaoCredentialVerifier interface {
	VerifyExternalCredential(ctx context.Context, email, password string) (*service.ExternalCredentialUser, error)
}

type InternalDujiaoAuthHandler struct {
	sharedSecret string
	verifier     DujiaoCredentialVerifier
}

func NewInternalDujiaoAuthHandler(cfg *config.Config, verifier DujiaoCredentialVerifier) *InternalDujiaoAuthHandler {
	if cfg == nil || !cfg.DujiaoLogin.Enabled || verifier == nil {
		return nil
	}
	secret := strings.TrimSpace(cfg.DujiaoLogin.SharedSecret)
	if secret == "" {
		return nil
	}
	return &InternalDujiaoAuthHandler{
		sharedSecret: secret,
		verifier:     verifier,
	}
}

type internalDujiaoVerifyRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type internalDujiaoVerifyResponse struct {
	User *service.ExternalCredentialUser `json:"user"`
}

func (h *InternalDujiaoAuthHandler) Verify(c *gin.Context) {
	if h == nil || h.verifier == nil || !h.hasValidSecret(c.GetHeader(internalDujiaoSecretHeader)) {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req internalDujiaoVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	user, err := h.verifier.VerifyExternalCredential(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials),
			errors.Is(err, service.ErrExternalLogin2FARequired),
			errors.Is(err, service.ErrUserNotActive):
			response.Unauthorized(c, "invalid email or password")
		default:
			response.ErrorFrom(c, err)
		}
		return
	}
	if user == nil {
		response.Unauthorized(c, "invalid email or password")
		return
	}

	response.Success(c, internalDujiaoVerifyResponse{User: user})
}

func (h *InternalDujiaoAuthHandler) hasValidSecret(provided string) bool {
	if strings.TrimSpace(provided) == "" || h.sharedSecret == "" {
		return false
	}
	providedSum := sha256.Sum256([]byte(provided))
	expectedSum := sha256.Sum256([]byte(h.sharedSecret))
	return subtle.ConstantTimeCompare(providedSum[:], expectedSum[:]) == 1
}
