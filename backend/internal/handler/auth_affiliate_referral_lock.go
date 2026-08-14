package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const (
	affiliateReferralLockCookieName    = "affiliate_referral_lock"
	affiliateReferralLockVersion       = 1
	affiliateReferralLockMaxAgeSeconds = 30 * 24 * 60 * 60
	affiliateReferralLockDomain        = "sub2api/affiliate-referral-lock/v1"
)

var errAffiliateReferralLockInvalid = errors.New("invalid affiliate referral lock")

type affiliateReferralLockPayload struct {
	Version   int    `json:"v"`
	Code      string `json:"code"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

type affiliateReferralResolveRequest struct {
	AffCode string `json:"aff_code" binding:"required"`
}

type affiliateReferralResolveResponse struct {
	Valid  bool `json:"valid"`
	Locked bool `json:"locked"`
}

type affiliateReferralStatusResponse struct {
	Locked bool `json:"locked"`
}

func affiliateReferralSigningKey(secret string) ([]byte, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errAffiliateReferralLockInvalid
	}
	sum := sha256.Sum256([]byte(affiliateReferralLockDomain + "\x00" + secret))
	return sum[:], nil
}

func encodeAffiliateReferralLock(secret, code string, issuedAt, expiresAt time.Time) (string, error) {
	return encodeAffiliateReferralLockVersion(secret, code, issuedAt, expiresAt, affiliateReferralLockVersion)
}

func encodeAffiliateReferralLockVersion(secret, code string, issuedAt, expiresAt time.Time, version int) (string, error) {
	key, err := affiliateReferralSigningKey(secret)
	if err != nil {
		return "", err
	}
	normalizedCode := strings.ToUpper(strings.TrimSpace(code))
	if normalizedCode == "" || !expiresAt.After(issuedAt) {
		return "", errAffiliateReferralLockInvalid
	}
	payload, err := json.Marshal(affiliateReferralLockPayload{
		Version:   version,
		Code:      normalizedCode,
		IssuedAt:  issuedAt.Unix(),
		ExpiresAt: expiresAt.Unix(),
	})
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(encodedPayload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encodedPayload + "." + signature, nil
}

func decodeAffiliateReferralLock(secret, value string, now time.Time) (string, error) {
	key, err := affiliateReferralSigningKey(secret)
	if err != nil {
		return "", err
	}
	parts := strings.Split(value, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", errAffiliateReferralLockInvalid
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errAffiliateReferralLockInvalid
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return "", errAffiliateReferralLockInvalid
	}
	rawPayload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", errAffiliateReferralLockInvalid
	}
	var payload affiliateReferralLockPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return "", errAffiliateReferralLockInvalid
	}
	code := strings.ToUpper(strings.TrimSpace(payload.Code))
	if payload.Version != affiliateReferralLockVersion ||
		code == "" ||
		payload.IssuedAt <= 0 ||
		payload.ExpiresAt <= payload.IssuedAt ||
		time.Unix(payload.IssuedAt, 0).After(now.Add(time.Minute)) ||
		!now.Before(time.Unix(payload.ExpiresAt, 0)) {
		return "", errAffiliateReferralLockInvalid
	}
	return code, nil
}

func (h *AuthHandler) affiliateReferralSecret() string {
	if h == nil || h.cfg == nil {
		return ""
	}
	return h.cfg.JWT.Secret
}

func (h *AuthHandler) readAffiliateReferralLock(c *gin.Context) (string, bool, error) {
	if c == nil || c.Request == nil {
		return "", false, nil
	}
	cookie, err := c.Request.Cookie(affiliateReferralLockCookieName)
	if errors.Is(err, http.ErrNoCookie) {
		return "", false, nil
	}
	if err != nil {
		return "", true, errAffiliateReferralLockInvalid
	}
	code, err := decodeAffiliateReferralLock(h.affiliateReferralSecret(), cookie.Value, time.Now())
	if err != nil {
		return "", true, err
	}
	return code, true, nil
}

func (h *AuthHandler) setAffiliateReferralLock(c *gin.Context, code string) error {
	now := time.Now()
	expiresAt := now.Add(time.Duration(affiliateReferralLockMaxAgeSeconds) * time.Second)
	value, err := encodeAffiliateReferralLock(h.affiliateReferralSecret(), code, now, expiresAt)
	if err != nil {
		return err
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     affiliateReferralLockCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   affiliateReferralLockMaxAgeSeconds,
		HttpOnly: true,
		Secure:   isRequestHTTPS(c),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (h *AuthHandler) clearAffiliateReferralLock(c *gin.Context) {
	if c == nil {
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     affiliateReferralLockCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isRequestHTTPS(c),
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) affiliateCodeForRequest(c *gin.Context, submitted string) (string, bool) {
	if code, present, err := h.readAffiliateReferralLock(c); present && err == nil {
		return code, true
	}
	return strings.TrimSpace(submitted), false
}

func (h *AuthHandler) ResolveAffiliateReferral(c *gin.Context) {
	var req affiliateReferralResolveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	code, err := h.authService.ResolveAffiliateReferralCode(c.Request.Context(), req.AffCode)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := h.setAffiliateReferralLock(c, code); err != nil {
		response.InternalError(c, "Failed to lock affiliate referral")
		return
	}
	response.Success(c, affiliateReferralResolveResponse{Valid: true, Locked: true})
}

func (h *AuthHandler) GetAffiliateReferralStatus(c *gin.Context) {
	_, present, err := h.readAffiliateReferralLock(c)
	if err != nil {
		h.clearAffiliateReferralLock(c)
		response.Success(c, affiliateReferralStatusResponse{Locked: false})
		return
	}
	response.Success(c, affiliateReferralStatusResponse{Locked: present})
}
