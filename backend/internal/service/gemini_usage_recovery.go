package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	geminiUsageRecoveryTimeout    = 4 * time.Second
	geminiUsageLedgerMaxBodyBytes = 4 << 20
)

var (
	ErrGeminiUsageRecoveryUnavailable     = errors.New("upstream usage ledger unavailable")
	ErrGeminiUsageRecoveryIdentityChanged = errors.New("upstream usage ledger identity changed")
	geminiUsageLedgerFetchGroup           singleflight.Group
)

// usageFollowupBudget keeps lookup/count work inside the original usage-task
// deadline and leaves time for its durable billing write. A client cancellation
// without a deadline may still use the existing bounded detached follow-up.
func usageFollowupBudget(ctx context.Context, maximum time.Duration) time.Duration {
	if ctx != nil {
		if deadline, ok := ctx.Deadline(); ok {
			return min(maximum, time.Until(deadline)-time.Second)
		}
	}
	return maximum
}

// GeminiUsageLedgerIdentity binds ledger recovery to the origin and exact API
// key actually used for inference, without retaining the key in usage metadata.
func GeminiUsageLedgerIdentity(rawURL, apiKey string) string {
	origin, ok := geminiUsageLedgerOrigin(rawURL)
	if !ok || apiKey == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(origin + "\x00" + apiKey))
	return hex.EncodeToString(digest[:])
}

func geminiUsageLedgerOrigin(rawURL string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.User != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", false
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host), true
}

// RecoverGeminiUsage fills missing provider measurements without changing any
// wallet, quota, dedup, or stored usage row.
func (s *GatewayService) RecoverGeminiUsage(ctx context.Context, input *RecordUsageInput) (bool, error) {
	if input == nil || input.Account == nil || !NeedsUsageMeasurementReview(input.Result) {
		return false, nil
	}
	account, result := input.Account, input.Result
	if account.Platform != PlatformGemini || account.Type != AccountTypeAPIKey ||
		strings.TrimSpace(result.UsageLedgerRequestID) == "" || len(result.UsageLedgerRequestID) > 128 || result.UsageLedgerIdentityHash == "" {
		return false, nil
	}
	apiKey := account.GetCredential("api_key")
	baseURL := account.GetCredential("base_url")
	identity := GeminiUsageLedgerIdentity(baseURL, apiKey)
	if identity == "" || identity != result.UsageLedgerIdentityHash {
		return false, ErrGeminiUsageRecoveryIdentityChanged
	}
	if s == nil || s.httpUpstream == nil || s.cfg == nil {
		return false, ErrGeminiUsageRecoveryUnavailable
	}
	origin, _ := geminiUsageLedgerOrigin(baseURL)
	normalizedOrigin, err := s.validateUpstreamBaseURL(origin)
	if err != nil {
		return false, ErrGeminiUsageRecoveryUnavailable
	}
	proxyURL := ""
	if account.ProxyID != nil {
		if account.Proxy == nil || account.Proxy.ID != *account.ProxyID {
			return false, ErrGeminiUsageRecoveryIdentityChanged
		}
		proxyURL = account.Proxy.URL()
	}
	model := strings.TrimSpace(result.UpstreamModel)
	if model == "" {
		model = strings.TrimSpace(result.Model)
	}
	if model == "" {
		return false, ErrGeminiUsageRecoveryUnavailable
	}
	budget := usageFollowupBudget(ctx, geminiUsageRecoveryTimeout)
	if budget <= 0 {
		return false, ErrGeminiUsageRecoveryUnavailable
	}
	baseContext := context.Background()
	if ctx != nil {
		baseContext = context.WithoutCancel(ctx)
	}
	lookupContext, cancel := context.WithTimeout(baseContext, budget)
	defer cancel()
	endpoint := strings.TrimRight(normalizedOrigin, "/") + "/api/log/token"
	// Coalesce overlapping reads of the same credential's ledger. Scope by the
	// service and proxy as well, so transport/security policies cannot cross over.
	proxyHash := sha256.Sum256([]byte(proxyURL))
	fetchKey := fmt.Sprintf("%p:%s:%x", s, identity, proxyHash)
	for _, delay := range []time.Duration{0, 250 * time.Millisecond, 750 * time.Millisecond, 1500 * time.Millisecond} {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-lookupContext.Done():
				timer.Stop()
				return false, ErrGeminiUsageRecoveryUnavailable
			case <-timer.C:
			}
		}
		fetch := geminiUsageLedgerFetchGroup.DoChan(fetchKey, func() (any, error) {
			return s.fetchGeminiUsageLedger(lookupContext, endpoint, apiKey, proxyURL, account)
		})
		var raw []byte
		select {
		case <-lookupContext.Done():
			return false, ErrGeminiUsageRecoveryUnavailable
		case fetched := <-fetch:
			if fetched.Err != nil {
				return false, ErrGeminiUsageRecoveryUnavailable
			}
			raw, _ = fetched.Val.([]byte)
		}
		usage, err := lookupGeminiLedgerUsage(raw, result.UsageLedgerRequestID, model)
		if err != nil {
			return false, ErrGeminiLedgerInvalidResponse
		}
		if usage == nil {
			continue
		}
		// Gemini stores exclusive input/cache buckets. Preserve an already
		// measured gross prompt (including explicit zero), then only fill absent
		// cache/output fields. A ledger is not permission to replace live usage.
		cacheReported := result.Usage.CacheReadInputTokens > 0 ||
			(result.GeminiUsageFallback != nil && result.GeminiUsageFallback.UsageFields&8 != 0)
		grossPrompt := result.Usage.InputTokens + result.Usage.CacheReadInputTokens
		if result.InputTokensSource != UsageTokenSourceUpstream {
			grossPrompt = usage.InputTokens + usage.CacheReadInputTokens
		}
		if !cacheReported && usage.CacheReadInputTokens <= grossPrompt {
			result.Usage.CacheReadInputTokens = usage.CacheReadInputTokens
		}
		if grossPrompt >= result.Usage.CacheReadInputTokens {
			result.Usage.InputTokens = grossPrompt - result.Usage.CacheReadInputTokens
			result.InputTokensSource = UsageTokenSourceUpstream
			result.EstimatedInputTokens, result.InputTokensEstimated = 0, false
		}
		if result.OutputTokensSource != UsageTokenSourceUpstream {
			result.Usage.OutputTokens = usage.OutputTokens
			result.OutputTokensSource = UsageTokenSourceUpstream
		}
		result.RequireMeasuredUsage = true
		result.MeasuredUsageComplete = result.InputTokensSource == UsageTokenSourceUpstream && result.OutputTokensSource == UsageTokenSourceUpstream
		return true, nil
	}
	return false, nil
}

func (s *GatewayService) fetchGeminiUsageLedger(ctx context.Context, endpoint, apiKey, proxyURL string, account *Account) ([]byte, error) {
	req, err := http.NewRequestWithContext(WithHTTPUpstreamRedirectsDisabled(ctx), http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, ErrGeminiUsageRecoveryUnavailable
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	// Do not apply inference header overrides: they may replace this credential
	// or Host. The exact captured origin/key is the authorization boundary here.
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil, ErrGeminiUsageRecoveryUnavailable
	}
	if resp == nil || resp.Body == nil {
		return nil, ErrGeminiUsageRecoveryUnavailable
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK || (resp.Request != nil && (resp.Request.URL == nil || resp.Request.URL.String() != endpoint)) {
		return nil, ErrGeminiUsageRecoveryUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, geminiUsageLedgerMaxBodyBytes+1))
	if err != nil || len(raw) > geminiUsageLedgerMaxBodyBytes {
		return nil, ErrGeminiUsageRecoveryUnavailable
	}
	return raw, nil
}
