package service

import "errors"

// ContinuationUsageAudit is the bounded, private settlement evidence for an
// individual provider call. It never contains a prompt, generated content,
// request/response headers, URL or credential. Ordinary DTOs do not serialize it.
type ContinuationUsageAudit struct {
	RequestID          string `json:"request_id"`
	LedgerRequestID    string `json:"ledger_request_id,omitempty"`
	LedgerIdentityHash string `json:"ledger_identity_hash,omitempty"`
	Model              string `json:"model"`
	UpstreamModel      string `json:"upstream_model,omitempty"`
	InputTokens        int    `json:"input_tokens"`
	// Populated on standalone protected settlements. OpenAI reports gross
	// input, while Messages and normalized Gemini exclude cache reads/writes.
	InputIncludesCache     bool             `json:"input_includes_cache,omitempty"`
	OutputTokens           int              `json:"output_tokens"`
	CacheReadTokens        int              `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens    int              `json:"cache_creation_tokens,omitempty"`
	CacheCreation5mTokens  int              `json:"cache_creation_5m_tokens,omitempty"`
	CacheCreation1hTokens  int              `json:"cache_creation_1h_tokens,omitempty"`
	CacheCreation30mTokens int              `json:"cache_creation_30m_tokens,omitempty"`
	CacheCreation12hTokens int              `json:"cache_creation_12h_tokens,omitempty"`
	ImageInputTokens       int              `json:"image_input_tokens,omitempty"`
	ImageCacheReadTokens   int              `json:"image_cache_read_tokens,omitempty"`
	CustomerInputTokens    int              `json:"customer_input_tokens"`
	CustomerOutputTokens   int              `json:"customer_output_tokens"`
	DeliveredOutputTokens  *int             `json:"delivered_output_tokens,omitempty"`
	InputTokensSource      UsageTokenSource `json:"input_tokens_source"`
	OutputTokensSource     UsageTokenSource `json:"output_tokens_source"`
	ServiceTier            string           `json:"service_tier,omitempty"`
	ResponseServiceTier    string           `json:"response_service_tier,omitempty"`
	InputTokenPolicy       string           `json:"input_token_policy,omitempty"`
	InputTokenPolicyCount  int              `json:"input_token_policy_count,omitempty"`
}

func ValidateContinuationUsageAudit(rounds []ContinuationUsageAudit) error {
	if len(rounds) > maxContinuationUsageResults {
		return ErrInvalidContinuationUsage
	}
	for _, r := range rounds {
		if len(r.RequestID) > 512 || len(r.LedgerRequestID) > 512 || len(r.LedgerIdentityHash) > 128 || len(r.Model) > 512 || len(r.UpstreamModel) > 512 || len(r.ServiceTier) > 64 || len(r.ResponseServiceTier) > 64 || len(r.InputTokenPolicy) > 64 {
			return ErrInvalidContinuationUsage
		}
		counts := []int{r.InputTokens, r.OutputTokens, r.CacheReadTokens, r.CacheCreationTokens, r.CacheCreation5mTokens, r.CacheCreation1hTokens, r.CacheCreation30mTokens, r.CacheCreation12hTokens, r.ImageInputTokens, r.ImageCacheReadTokens, r.CustomerInputTokens, r.CustomerOutputTokens, r.InputTokenPolicyCount}
		if r.DeliveredOutputTokens != nil {
			counts = append(counts, *r.DeliveredOutputTokens)
		}
		if !validContinuationTokens(counts, make([]int, len(counts))) {
			return ErrInvalidContinuationUsage
		}
		if r.InputTokensSource.Normalize() != r.InputTokensSource || r.OutputTokensSource.Normalize() != r.OutputTokensSource {
			return errors.New("invalid continuation token provenance")
		}
	}
	return nil
}

func forwardContinuationAudit(result, projected *ForwardResult, protectionApplied bool) []ContinuationUsageAudit {
	if len(result.ContinuationResults) == 0 && !protectionApplied {
		return nil
	}
	rawRounds, customerRounds := forwardBillingRounds(result), forwardBillingRounds(projected)
	rounds := make([]ContinuationUsageAudit, 0, len(rawRounds))
	for i, r := range rawRounds {
		customer := customerRounds[i]
		audit := ContinuationUsageAudit{
			InputTokens: r.Usage.InputTokens, OutputTokens: r.Usage.OutputTokens,
			CacheReadTokens: r.Usage.CacheReadInputTokens, CacheCreationTokens: r.Usage.CacheCreationInputTokens,
			CacheCreation5mTokens: r.Usage.CacheCreation5mTokens, CacheCreation1hTokens: r.Usage.CacheCreation1hTokens,
			CustomerInputTokens: customer.Usage.InputTokens, CustomerOutputTokens: customer.Usage.OutputTokens,
			InputTokensSource: r.InputTokensSource.Normalize(), OutputTokensSource: r.OutputTokensSource.Normalize(),
		}
		// A standalone call already has its identity on the usage row.
		if len(result.ContinuationResults) != 0 {
			audit.RequestID, audit.Model, audit.UpstreamModel = r.RequestID, r.Model, r.UpstreamModel
			audit.LedgerRequestID, audit.LedgerIdentityHash = r.UsageLedgerRequestID, r.UsageLedgerIdentityHash
			audit.ServiceTier, audit.ResponseServiceTier = optionalStringValue(r.ServiceTier), r.UpstreamResponseServiceTier
			audit.DeliveredOutputTokens = r.DeliveredOutputTokens
		}
		if p := customer.InputTokenProtection; p != nil && protectionApplied {
			audit.InputTokenPolicy, audit.InputTokenPolicyCount = "group_input_token_protection", p.Limit
		} else if r.GeminiInputCounting != nil && projected.InputTokenProtection == nil {
			audit.InputTokenPolicy = r.GeminiInputCounting.Policy
			audit.InputTokenPolicyCount = r.GeminiInputCounting.CandidateInputTokens
		}
		rounds = append(rounds, audit)
	}
	return rounds
}

func openAIContinuationAudit(result *OpenAIForwardResult) []ContinuationUsageAudit {
	if len(result.ContinuationResults) == 0 {
		return nil
	}
	rounds := make([]ContinuationUsageAudit, 0, len(result.ContinuationResults))
	for _, r := range result.ContinuationResults {
		customer := openAIContinuationUsageTokens(r, true)
		rounds = append(rounds, ContinuationUsageAudit{RequestID: r.RequestID, Model: r.Model, UpstreamModel: r.UpstreamModel,
			InputTokens: r.Usage.InputTokens, OutputTokens: r.Usage.OutputTokens, CacheReadTokens: r.Usage.CacheReadInputTokens, CacheCreationTokens: r.Usage.CacheCreationInputTokens,
			CustomerInputTokens: customer.InputTokens, CustomerOutputTokens: customer.OutputTokens, DeliveredOutputTokens: r.DeliveredOutputTokens,
			InputTokensSource: r.InputTokensSource.Normalize(), OutputTokensSource: r.OutputTokensSource.Normalize(), ServiceTier: optionalStringValue(r.ServiceTier), ResponseServiceTier: r.UpstreamResponseServiceTier})
	}
	return rounds
}

// OpenAI reports gross prompt input. Copy the provider's counters before any
// disjoint-bucket pricing or customer cap, including the native write TTLs.
func openAIProtectedSingleCallAudit(r *OpenAIForwardResult, customer UsageTokens, limit int) ContinuationUsageAudit {
	u := r.Usage
	return ContinuationUsageAudit{
		InputIncludesCache: true, InputTokenPolicy: "group_input_token_protection", InputTokenPolicyCount: limit,
		InputTokens: u.InputTokens, OutputTokens: u.OutputTokens,
		CacheReadTokens: u.CacheReadInputTokens, CacheCreationTokens: u.CacheCreationInputTokens,
		CacheCreation5mTokens: u.CacheCreation5mTokens, CacheCreation1hTokens: u.CacheCreation1hTokens,
		CacheCreation30mTokens: u.CacheCreation30mInputTokens, CacheCreation12hTokens: u.CacheCreation12hInputTokens,
		ImageInputTokens: u.ImageInputTokens, ImageCacheReadTokens: u.ImageCacheReadTokens,
		CustomerInputTokens: customer.InputTokens, CustomerOutputTokens: customer.OutputTokens,
		InputTokensSource: r.InputTokensSource.Normalize(), OutputTokensSource: r.OutputTokensSource.Normalize(),
	}
}

func withContinuationAudit(outcome *ResponseOutcome, rounds []ContinuationUsageAudit) *ResponseOutcome {
	if len(rounds) == 0 {
		return outcome
	}
	var snapshot ResponseOutcome
	if outcome != nil {
		snapshot = *outcome
	}
	snapshot.ContinuationUsage = rounds
	return &snapshot
}
