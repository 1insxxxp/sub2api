package service

import "errors"

// ContinuationUsageAudit is the bounded, private settlement evidence for an
// individual provider call. It never contains a prompt, generated content,
// request/response headers, URL or credential. Ordinary DTOs do not serialize it.
type ContinuationUsageAudit struct {
	RequestID             string           `json:"request_id"`
	LedgerRequestID       string           `json:"ledger_request_id,omitempty"`
	LedgerIdentityHash    string           `json:"ledger_identity_hash,omitempty"`
	Model                 string           `json:"model"`
	UpstreamModel         string           `json:"upstream_model,omitempty"`
	InputTokens           int              `json:"input_tokens"`
	OutputTokens          int              `json:"output_tokens"`
	CacheReadTokens       int              `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens   int              `json:"cache_creation_tokens,omitempty"`
	CacheCreation5mTokens int              `json:"cache_creation_5m_tokens,omitempty"`
	CacheCreation1hTokens int              `json:"cache_creation_1h_tokens,omitempty"`
	CustomerInputTokens   int              `json:"customer_input_tokens"`
	CustomerOutputTokens  int              `json:"customer_output_tokens"`
	DeliveredOutputTokens *int             `json:"delivered_output_tokens,omitempty"`
	InputTokensSource     UsageTokenSource `json:"input_tokens_source"`
	OutputTokensSource    UsageTokenSource `json:"output_tokens_source"`
	ServiceTier           string           `json:"service_tier,omitempty"`
	ResponseServiceTier   string           `json:"response_service_tier,omitempty"`
	InputTokenPolicy      string           `json:"input_token_policy,omitempty"`
	InputTokenPolicyCount int              `json:"input_token_policy_count,omitempty"`
}

func ValidateContinuationUsageAudit(rounds []ContinuationUsageAudit) error {
	if len(rounds) > maxContinuationUsageResults {
		return ErrInvalidContinuationUsage
	}
	for _, r := range rounds {
		if len(r.RequestID) > 512 || len(r.LedgerRequestID) > 512 || len(r.LedgerIdentityHash) > 128 || len(r.Model) > 512 || len(r.UpstreamModel) > 512 || len(r.ServiceTier) > 64 || len(r.ResponseServiceTier) > 64 || len(r.InputTokenPolicy) > 64 {
			return ErrInvalidContinuationUsage
		}
		counts := []int{r.InputTokens, r.OutputTokens, r.CacheReadTokens, r.CacheCreationTokens, r.CacheCreation5mTokens, r.CacheCreation1hTokens, r.CustomerInputTokens, r.CustomerOutputTokens, r.InputTokenPolicyCount}
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

func forwardContinuationAudit(result *ForwardResult, account *Account) []ContinuationUsageAudit {
	if len(result.ContinuationResults) == 0 {
		return nil
	}
	rounds := make([]ContinuationUsageAudit, 0, len(result.ContinuationResults))
	for _, r := range result.ContinuationResults {
		customer := continuationCustomerForwardLeaf(r, account, true)
		audit := ContinuationUsageAudit{RequestID: r.RequestID, LedgerRequestID: r.UsageLedgerRequestID, LedgerIdentityHash: r.UsageLedgerIdentityHash, Model: r.Model, UpstreamModel: r.UpstreamModel,
			InputTokens: r.Usage.InputTokens, OutputTokens: r.Usage.OutputTokens, CacheReadTokens: r.Usage.CacheReadInputTokens, CacheCreationTokens: r.Usage.CacheCreationInputTokens, CacheCreation5mTokens: r.Usage.CacheCreation5mTokens, CacheCreation1hTokens: r.Usage.CacheCreation1hTokens,
			CustomerInputTokens: customer.Usage.InputTokens, CustomerOutputTokens: customer.Usage.OutputTokens, DeliveredOutputTokens: r.DeliveredOutputTokens,
			InputTokensSource: r.InputTokensSource.Normalize(), OutputTokensSource: r.OutputTokensSource.Normalize(), ServiceTier: optionalStringValue(r.ServiceTier), ResponseServiceTier: r.UpstreamResponseServiceTier}
		if r.GeminiInputCounting != nil {
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
