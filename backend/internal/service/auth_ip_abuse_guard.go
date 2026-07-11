package service

import (
	"context"
	"net"
	"strings"
	"time"
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

type AuthIPAbuseCounter interface {
	Increment(ctx context.Context, event AuthIPAbuseEvent, ip string, window time.Duration) (int64, error)
}

type AuthIPAbuseGuard struct {
	counter    AuthIPAbuseCounter
	settingSvc *SettingService
}

func NewAuthIPAbuseGuard(counter AuthIPAbuseCounter, settingSvc *SettingService) *AuthIPAbuseGuard {
	return &AuthIPAbuseGuard{counter: counter, settingSvc: settingSvc}
}

func (g *AuthIPAbuseGuard) Record(ctx context.Context, event AuthIPAbuseEvent, clientIP string) (*AuthIPAbuseDecision, error) {
	decision := &AuthIPAbuseDecision{Event: string(event), IP: strings.TrimSpace(clientIP)}
	if g == nil || g.counter == nil || g.settingSvc == nil {
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

	count, err := g.counter.Increment(ctx, event, decision.IP, window)
	if err != nil {
		return decision, err
	}
	decision.Count = count
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
