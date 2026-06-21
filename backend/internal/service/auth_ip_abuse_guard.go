package service

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type AuthIPAbuseEvent string

const (
	AuthIPAbuseEventRegisterSuccess AuthIPAbuseEvent = "register_success"
	AuthIPAbuseEventVerifyCodeSent  AuthIPAbuseEvent = "verify_code_sent"
	AuthIPAbuseEventLoginFailure    AuthIPAbuseEvent = "login_failure"
)

type AuthIPAbuseDecision struct {
	Blocked   bool
	Event     string
	IP        string
	Count     int64
	Threshold int64
}

type AuthIPAbuseGuard struct {
	redis      *redis.Client
	settingSvc *SettingService
}

func NewAuthIPAbuseGuard(redisClient *redis.Client, settingSvc *SettingService) *AuthIPAbuseGuard {
	return &AuthIPAbuseGuard{redis: redisClient, settingSvc: settingSvc}
}

func (g *AuthIPAbuseGuard) Record(ctx context.Context, event AuthIPAbuseEvent, clientIP string) (*AuthIPAbuseDecision, error) {
	decision := &AuthIPAbuseDecision{Event: string(event), IP: strings.TrimSpace(clientIP)}
	if g == nil || g.redis == nil || g.settingSvc == nil {
		return decision, nil
	}
	if net.ParseIP(decision.IP) == nil {
		return decision, nil
	}

	settings, err := g.settingSvc.GetAuthIPAutoBlockSettings(ctx)
	if err != nil {
		return decision, err
	}
	if settings == nil || !settings.Enabled {
		return decision, nil
	}

	threshold := authIPAbuseThreshold(settings, event)
	if threshold <= 0 {
		return decision, nil
	}
	decision.Threshold = int64(threshold)

	window := time.Duration(settings.WindowMinutes) * time.Minute
	if window <= 0 {
		window = time.Duration(defaultAuthIPAutoBlockWindowMinutes) * time.Minute
	}

	key := fmt.Sprintf("auth_ip_abuse:%s:%s", event, decision.IP)
	count, err := g.redis.Incr(ctx, key).Result()
	if err != nil {
		return decision, err
	}
	decision.Count = count
	if count == 1 {
		_ = g.redis.Expire(ctx, key, window).Err()
	}
	if count < decision.Threshold {
		return decision, nil
	}

	if err := g.settingSvc.AddAuthIPBlacklistRule(ctx, decision.IP); err != nil {
		return decision, err
	}
	decision.Blocked = true
	return decision, nil
}

func authIPAbuseThreshold(settings *AuthIPAutoBlockSettings, event AuthIPAbuseEvent) int {
	if settings == nil {
		return 0
	}
	switch event {
	case AuthIPAbuseEventRegisterSuccess:
		return settings.RegisterThreshold
	case AuthIPAbuseEventVerifyCodeSent:
		return settings.VerifyCodeThreshold
	case AuthIPAbuseEventLoginFailure:
		return settings.LoginFailureThreshold
	default:
		return 0
	}
}
