package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrRegistrationCoolingDown        = infraerrors.TooManyRequests("REGISTRATION_COOLDOWN", "当前注册过于频繁，请稍后重试")
	ErrRegistrationCaptchaRequired    = infraerrors.Forbidden("REGISTRATION_CAPTCHA_REQUIRED", "请先完成人机验证后重试注册")
	ErrRegistrationCaptchaUnavailable = infraerrors.ServiceUnavailable("REGISTRATION_CAPTCHA_UNAVAILABLE", "注册验证暂时不可用，请稍后重试")
)

func isRegistrationRiskError(err error) bool {
	return errors.Is(err, ErrRegistrationCoolingDown) || errors.Is(err, ErrRegistrationCaptchaRequired) || errors.Is(err, ErrRegistrationCaptchaUnavailable)
}

type registrationRiskContextKey struct{}

// RegistrationRiskSource exists for a single interactive authentication request.
// Only a verified client address and a server-generated browser hash are accepted.
type RegistrationRiskSource struct {
	IP          string
	UserAgent   string
	BrowserHash string
	OAuth       bool
	action      string
}

func WithRegistrationRiskSource(ctx context.Context, src *RegistrationRiskSource) context.Context {
	addr, err := netip.ParseAddr(src.IP)
	if err == nil {
		src.IP = addr.Unmap().String()
	} else {
		src.IP = ""
	}
	src.UserAgent = strings.ToValidUTF8(sanitizeAuthSourceUserAgent(strings.ReplaceAll(src.UserAgent, "\x00", "")), "")
	return context.WithValue(ctx, registrationRiskContextKey{}, src)
}
func RegistrationRiskSourceFrom(ctx context.Context) *RegistrationRiskSource {
	src, _ := ctx.Value(registrationRiskContextKey{}).(*RegistrationRiskSource)
	return src
}

func (source *RegistrationRiskSource) decision(ctx context.Context, client *dbent.Client, cfg AccountRiskConfig, now time.Time) (string, int, error) {
	rows, err := client.QueryContext(ctx, `SELECT EXISTS(SELECT 1 FROM account_registration_cooldowns WHERE ip_address=$1 AND expires_at>$2),
 (SELECT COUNT(*) FROM account_risk_events WHERE event_type='register' AND ip_address=$1 AND created_at>$3)`, source.IP, now, now.Add(-time.Duration(cfg.WindowMinutes)*time.Minute))
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = rows.Close() }()
	var cooldown bool
	var count int
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", 0, err
		}
		return "", 0, fmt.Errorf("missing registration window count")
	}
	if err = rows.Scan(&cooldown, &count); err != nil {
		return "", 0, err
	}
	if cooldown || count >= cfg.CooldownCount {
		return "cooldown", count, nil
	}
	if count >= cfg.ChallengeCount {
		return "challenge", count, nil
	}
	return "", count, nil
}

func registrationCaptchaScope(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
func (source *RegistrationRiskSource) saveCaptchaReceipt(ctx context.Context, client *dbent.Client, scope string, now time.Time) error {
	_, err := client.ExecContext(ctx, `INSERT INTO account_registration_captcha (ip_address,browser_hash,scope,expires_at) VALUES ($1,$2,$3,$4)
 ON CONFLICT (ip_address,browser_hash,scope) DO UPDATE SET expires_at=excluded.expires_at`, source.IP, source.BrowserHash, registrationCaptchaScope(scope), now.Add(15*time.Minute))
	return err
}

// A valid email code may already exist when the IP crosses the challenge
// threshold. Check its matching receipt before the email service consumes it.
func (source *RegistrationRiskSource) hasEmailReceipt(ctx context.Context, client *dbent.Client, email string, now time.Time) (bool, error) {
	rows, err := client.QueryContext(ctx, `SELECT 1 FROM account_registration_captcha WHERE ip_address=$1 AND browser_hash=$2 AND scope=$3 AND expires_at>$4 LIMIT 1`, source.IP, source.BrowserHash, registrationCaptchaScope(email), now)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	found := rows.Next()
	return found, rows.Err()
}

// CompleteRegistration runs in the user creation transaction, under the same IP's
// transaction-scoped advisory lock. Returning an error rolls the new user back.
func (source *RegistrationRiskSource) CompleteRegistration(ctx context.Context, client *dbent.Client, cfg AccountRiskConfig, userID int64, email string, now time.Time) error {
	if source.IP == "" || source.BrowserHash == "" {
		return ErrServiceUnavailable
	}
	action, count, err := source.decision(ctx, client, cfg, now)
	if err != nil {
		return err
	}
	source.action = action
	if cfg.RegistrationMode == "enforce" && action == "cooldown" {
		return ErrRegistrationCoolingDown
	}
	scope := registrationCaptchaScope(email)
	oauthScope := scope
	if source.OAuth {
		oauthScope = "@oauth"
	}
	result, err := client.ExecContext(ctx, `DELETE FROM account_registration_captcha WHERE ip_address=$1 AND browser_hash=$2 AND (scope=$3 OR scope=$4) AND expires_at>$5`, source.IP, source.BrowserHash, scope, oauthScope, now)
	if err != nil {
		return err
	}
	accepted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if cfg.RegistrationMode == "enforce" && action == "challenge" && accepted == 0 {
		return ErrRegistrationCaptchaRequired
	}
	if count+1 >= cfg.CooldownCount {
		_, err = client.ExecContext(ctx, `INSERT INTO account_registration_cooldowns (ip_address,expires_at) VALUES ($1,$2)
   ON CONFLICT(ip_address) DO UPDATE SET expires_at=excluded.expires_at`, source.IP, now.Add(time.Duration(cfg.CooldownMinutes)*time.Minute))
		if err != nil {
			return err
		}
	}
	return source.recordEvent(ctx, client, userID, "register", now)
}
func (source *RegistrationRiskSource) recordEvent(ctx context.Context, client *dbent.Client, userID int64, event string, now time.Time) error {
	_, err := client.ExecContext(ctx, `INSERT INTO account_risk_events(user_id,event_type,browser_hash,ip_address,user_agent,created_at) VALUES ($1,$2,$3,$4,$5,$6)`, userID, event, source.BrowserHash, source.IP, source.UserAgent, now)
	return err
}

// FinishRegistrationRiskRequest counts a decision after the request transaction
// ends, including blocked attempts whose user creation was rolled back.
func (s *AuthService) FinishRegistrationRiskRequest(ctx context.Context) {
	src := RegistrationRiskSourceFrom(ctx)
	if src == nil || src.action == "" || s == nil || s.entClient == nil {
		return
	}
	day := time.Now().In(time.FixedZone("Asia/Shanghai", 8*3600)).Format(time.DateOnly)
	_, err := s.entClient.ExecContext(ctx, `INSERT INTO account_registration_risk_counts(day,action,hits) VALUES ($1,$2,1) ON CONFLICT(day,action) DO UPDATE SET hits=account_registration_risk_counts.hits+1`, day, src.action)
	if err != nil {
		slog.Warn("failed to count registration risk decision", "error", err)
	}
}
func (s *CheckinService) RegistrationRiskHitsToday(ctx context.Context) (map[string]int64, error) {
	day, _ := s.currentBeijingDay()
	rows, err := s.entClient.QueryContext(ctx, `SELECT action,hits FROM account_registration_risk_counts WHERE day=$1`, day)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	hits := map[string]int64{"challenge": 0, "cooldown": 0}
	for rows.Next() {
		var action string
		var n int64
		if err = rows.Scan(&action, &n); err != nil {
			return nil, err
		}
		hits[action] = n
	}
	return hits, rows.Err()
}

// A globally enabled provider keeps its existing behavior. When global CAPTCHA
// is disabled, exactly one fully configured provider can serve risk challenges.
func registrationCaptchaConfig(config CaptchaProviderConfig) (CaptchaProviderConfig, bool) {
	if config.TurnstileEnabled || config.Tencent.Enabled || config.Aliyun.Enabled {
		return config, !captchaProvidersConflict(config.TurnstileEnabled, config.Tencent.Enabled, config.Aliyun.Enabled)
	}
	config.TurnstileEnabled = strings.TrimSpace(config.TurnstileSecretKey) != "" && strings.TrimSpace(config.TurnstileSiteKey) != ""
	_, config.Tencent.Enabled = parseTencentCaptchaCredentials(config.Tencent)
	_, aliyunReady := aliyunCaptchaCredentials(config.Aliyun)
	config.Aliyun.Enabled = aliyunReady && strings.TrimSpace(config.AliyunPrefix) != ""
	return config, (config.TurnstileEnabled || config.Tencent.Enabled || config.Aliyun.Enabled) && !captchaProvidersConflict(config.TurnstileEnabled, config.Tencent.Enabled, config.Aliyun.Enabled)
}

// SignupChallengeProvider returns only the widget selector. The site key,
// app ID, region and other public widget parameters already ship in settings.
func (s *AuthService) SignupChallengeProvider(ctx context.Context) (string, error) {
	src := RegistrationRiskSourceFrom(ctx)
	if src == nil || s.entClient == nil || s.settingService == nil {
		return "", nil
	}
	cfg, err := ReadAccountRiskConfig(ctx, s.settingService.settingRepo)
	if err != nil {
		return "", err
	}
	if cfg.RegistrationMode != "enforce" {
		return "", nil
	}
	action, _, err := src.decision(ctx, s.entClient, cfg, time.Now().UTC())
	if err != nil || action != "challenge" {
		return "", err
	}
	raw, err := s.settingService.GetCaptchaProviderConfig(ctx)
	if err != nil {
		return "", err
	}
	// A global CAPTCHA already renders from public settings.
	if raw.TurnstileEnabled || raw.Tencent.Enabled || raw.Aliyun.Enabled {
		return "", nil
	}
	chosen, available := registrationCaptchaConfig(raw)
	if !available {
		return "", nil
	}
	switch {
	case chosen.TurnstileEnabled:
		return "turnstile", nil
	case chosen.Tencent.Enabled:
		return "tencent", nil
	default:
		return "aliyun", nil
	}
}

func (s *AuthService) registrationRiskPublicError(ctx context.Context, err error) error {
	if !errors.Is(err, ErrRegistrationCaptchaRequired) {
		return err
	}
	providers, configErr := s.settingService.GetCaptchaProviderConfig(ctx)
	_, available := registrationCaptchaConfig(providers)
	if configErr != nil || !available {
		return ErrRegistrationCaptchaUnavailable
	}
	return err
}

// VerifySignupCaptcha preserves the normal provider checks and stores only a
// successfully verified proof. Email-code submissions rely on a receipt issued
// when the code was sent. The creation transaction checks and consumes it.
func (s *AuthService) VerifySignupCaptcha(ctx context.Context, proof CaptchaProof, remoteIP, email string, skipEmailProof bool) error {
	src := RegistrationRiskSourceFrom(ctx)
	if src == nil || s.entClient == nil || s.settingService == nil {
		if skipEmailProof {
			return nil
		}
		return s.VerifyCaptcha(ctx, proof, remoteIP)
	}
	cfg, err := ReadAccountRiskConfig(ctx, s.settingService.settingRepo)
	if err != nil {
		return ErrServiceUnavailable
	}
	action, _, err := src.decision(ctx, s.entClient, cfg, time.Now().UTC())
	if err != nil {
		return ErrServiceUnavailable
	}
	src.action = action
	if cfg.RegistrationMode == "enforce" && action == "cooldown" {
		return ErrRegistrationCoolingDown
	}
	if skipEmailProof {
		if cfg.RegistrationMode == "enforce" && action == "challenge" {
			granted, err := src.hasEmailReceipt(ctx, s.entClient, email, time.Now().UTC())
			if err != nil {
				return ErrServiceUnavailable
			}
			if !granted {
				return s.registrationRiskPublicError(ctx, ErrRegistrationCaptchaRequired)
			}
		}
		return nil
	}
	providers, err := s.settingService.GetCaptchaProviderConfig(ctx)
	if err != nil {
		return ErrServiceUnavailable
	}
	enabled := providers.TurnstileEnabled || providers.Tencent.Enabled || providers.Aliyun.Enabled
	challenged := cfg.RegistrationMode == "enforce" && action == "challenge"
	if challenged {
		providers, enabled = registrationCaptchaConfig(providers)
		if !enabled {
			return ErrRegistrationCaptchaUnavailable
		}
		if strings.TrimSpace(proof.TurnstileToken) == "" && strings.TrimSpace(proof.TencentTicket) == "" {
			return ErrRegistrationCaptchaRequired
		}
	}
	required := s.cfg != nil && s.cfg.Server.Mode == "release" && s.cfg.Turnstile.Required
	if err := s.verifyCaptchaWithConfig(ctx, providers, proof, src.IP, required); err != nil {
		if challenged && !isBadCaptchaProof(err) {
			return ErrRegistrationCaptchaUnavailable
		}
		return err
	}
	if enabled {
		return src.saveCaptchaReceipt(ctx, s.entClient, email, time.Now().UTC())
	}
	return nil
}

func isBadCaptchaProof(err error) bool {
	// Provider transport failures wrap their legacy bad-proof sentinel.
	return err == ErrTurnstileVerificationFailed || err == ErrTencentCaptchaVerificationFailed || err == ErrAliyunCaptchaVerificationFailed
}

// OAuth start may precede either a login or a signup. Keep existing login
// eligibility independent of registration risk; a validated ticket can be used
// for one new account from the same IP and browser during this flow.
func (s *AuthService) VerifyOAuthStartCaptcha(ctx context.Context, proof CaptchaProof, remoteIP string) error {
	src := RegistrationRiskSourceFrom(ctx)
	if src == nil || s.entClient == nil || s.settingService == nil {
		return s.VerifyActionCaptchaIfEnabled(ctx, proof, remoteIP)
	}
	providers, err := s.settingService.GetCaptchaProviderConfig(ctx)
	if err != nil {
		return ErrServiceUnavailable
	}
	// The risk-only widget submits a proof only for an intended OAuth signup.
	if !providers.TurnstileEnabled && !providers.Tencent.Enabled && !providers.Aliyun.Enabled && (proof.TurnstileToken != "" || proof.TencentTicket != "") {
		var available bool
		providers, available = registrationCaptchaConfig(providers)
		if !available {
			return ErrRegistrationCaptchaUnavailable
		}
	}
	if !providers.Tencent.Enabled && !providers.Aliyun.Enabled && (!providers.TurnstileEnabled || proof.TurnstileToken == "") {
		return nil
	}
	if err = s.verifyCaptchaWithConfig(ctx, providers, proof, src.IP, false); err != nil {
		return err
	}
	return src.saveCaptchaReceipt(ctx, s.entClient, "@oauth", time.Now().UTC())
}
