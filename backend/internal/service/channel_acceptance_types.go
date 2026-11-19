package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"
)

const ChannelAcceptanceRuleVersion = "channel-acceptance-v2"

var (
	ErrAcceptanceInvalid       = errors.New("验收参数无效")
	ErrAcceptanceNotFound      = errors.New("验收记录不存在")
	ErrAcceptanceStale         = errors.New("账号配置或价格已变化，请重新预览")
	ErrAcceptancePriceRequired = errors.New("请先补充上游成本价格")
)

type AcceptancePrice struct {
	CacheWritePerMillion float64 `json:"cache_write_per_million,omitempty"`
	InputPerMillion      float64 `json:"input_per_million"`
	OutputPerMillion     float64 `json:"output_per_million"`
	CachePerMillion      float64 `json:"cache_per_million"`
	Source               string  `json:"source"`
}

func (p *AcceptancePrice) UnmarshalJSON(data []byte) error {
	type plain AcceptancePrice
	var value plain
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&value); err != nil {
		return err
	}
	if value.Source == "manual" {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			return err
		}
		for _, key := range []string{"input_per_million", "output_per_million", "cache_per_million"} {
			v, ok := fields[key]
			if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
				return ErrAcceptancePriceRequired
			}
		}
	}
	*p = AcceptancePrice(value)
	return nil
}

type AcceptanceTargetRequest struct {
	AccountID int64            `json:"account_id"`
	Model     string           `json:"model"`
	GroupID   *int64           `json:"group_id,omitempty"`
	Price     *AcceptancePrice `json:"price,omitempty"`
}
type AcceptancePreviewRequest struct {
	Targets   []AcceptanceTargetRequest `json:"targets"`
	Extras    []string                  `json:"extras"`
	BudgetUSD *float64                  `json:"budget_usd,omitempty"`
}
type AcceptanceModel struct {
	UnavailableReason string           `json:"unavailable_reason,omitempty"`
	Model             string           `json:"model"`
	UpstreamModel     string           `json:"upstream_model"`
	Protocols         []string         `json:"protocols"`
	Price             *AcceptancePrice `json:"price,omitempty"`
}
type AcceptanceTarget struct {
	UnavailableReason string            `json:"unavailable_reason,omitempty"`
	AccountID         int64             `json:"account_id"`
	Name              string            `json:"name"`
	Platform          string            `json:"platform"`
	Models            []AcceptanceModel `json:"models"`
	Groups            []AcceptanceGroup `json:"groups"`
}

type AcceptanceGroup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Snapshot contains no credentials. Fingerprint binds all effective account settings.
type AcceptanceSnapshot struct {
	AccountType             string          `json:"account_type,omitempty"`
	Protocols               []string        `json:"protocols,omitempty"`
	AccountID               int64           `json:"account_id"`
	AccountName             string          `json:"account_name"`
	Platform                string          `json:"platform"`
	Model                   string          `json:"model"`
	UpstreamModel           string          `json:"upstream_model"`
	GroupID                 *int64          `json:"group_id,omitempty"`
	Fingerprint             string          `json:"fingerprint"`
	Price                   AcceptancePrice `json:"price"`
	AdditionalInputEstimate int             `json:"additional_input_estimate"`
}
type AcceptanceSample struct {
	ID               string             `json:"id"`
	RunID            string             `json:"run_id,omitempty"`
	Ordinal          int                `json:"ordinal"`
	Snapshot         AcceptanceSnapshot `json:"target"`
	Category         string             `json:"category"`
	Protocol         string             `json:"protocol"` // gemini / messages / responses / chat / count_tokens
	Stream           bool               `json:"stream"`
	Prompt           string             `json:"prompt"` // synthetic only; never user conversations
	Expected         string             `json:"expected,omitempty"`
	InputSize        int                `json:"input_size"`
	MaxOutputTokens  int                `json:"max_output_tokens"`
	TimeoutSeconds   int                `json:"timeout_seconds"`
	EstimatedCostUSD float64            `json:"estimated_cost_usd"`
	Status           string             `json:"status"`
	LeaseToken       string             `json:"-"`
	LeaseUntil       *time.Time         `json:"-"`
	Result           *AcceptanceResult  `json:"result,omitempty"`
}
type AcceptancePreview struct {
	ID               string                   `json:"id"`
	ActorID          int64                    `json:"-"`
	RuleVersion      string                   `json:"rule_version"`
	CreatedAt        time.Time                `json:"created_at"`
	ExpiresAt        time.Time                `json:"expires_at"`
	Request          AcceptancePreviewRequest `json:"request"`
	Samples          []AcceptanceSample       `json:"samples"`
	GenerationCalls  int                      `json:"generation_calls"`
	CountCalls       int                      `json:"count_calls"`
	EstimatedCostUSD float64                  `json:"estimated_cost_usd"`
	CanStart         bool                     `json:"can_start"`
	Issues           []string                 `json:"issues"`
}
type AcceptanceUsage struct {
	CacheCreate            *int64 `json:"cache_create,omitempty"`
	OutputIncludesThoughts bool   `json:"output_includes_thoughts"`
	Input                  *int64 `json:"input"`
	Output                 *int64 `json:"output"`
	Thoughts               *int64 `json:"thoughts"`
	CacheRead              *int64 `json:"cache_read"`
	Total                  *int64 `json:"total"`
	Source                 string `json:"source"`
}
type AcceptanceSource struct {
	Kind           string `json:"kind"` // direct / declared / unverified
	Channel        string `json:"channel"`
	Host           string `json:"host"`
	Path           string `json:"path"`
	AuthKind       string `json:"auth_kind"`
	Adapter        string `json:"adapter"`
	RequestedModel string `json:"requested_model"`
	UpstreamModel  string `json:"upstream_model"`
	ResponseModel  string `json:"response_model,omitempty"`
}
type AcceptanceFinding struct {
	Code    string `json:"code"`
	Level   string `json:"level"` // info / warning / error
	Message string `json:"message"`
}
type AcceptanceResult struct {
	Status              string              `json:"status"` // passed / anomaly / unsupported / inconclusive / error / cancelled / unknown
	Source              AcceptanceSource    `json:"source"`
	RawUsage            AcceptanceUsage     `json:"raw_usage"`
	UpstreamUsageFinal  bool                `json:"upstream_usage_final"`
	ClientUsage         AcceptanceUsage     `json:"client_usage"`
	BillingUsage        AcceptanceUsage     `json:"billing_usage"`
	Findings            []AcceptanceFinding `json:"findings"`
	DurationMS          int64               `json:"duration_ms"`
	FirstTokenMS        *int64              `json:"first_token_ms,omitempty"`
	HTTPStatus          int                 `json:"http_status"`
	FinishReason        string              `json:"finish_reason"`
	Completed           bool                `json:"completed"`
	HasText             bool                `json:"has_text"`
	HasReasoning        bool                `json:"has_reasoning"`
	HasTool             bool                `json:"has_tool"`
	OutputExcerpt       string              `json:"output_excerpt"`
	UsageFrames         []json.RawMessage   `json:"usage_frames,omitempty"`
	EffectiveParameters map[string]any      `json:"effective_parameters,omitempty"`
	CostUSD             *float64            `json:"cost_usd"` // calculated from reported usage, never a claimed supplier invoice
}
type AcceptanceRun struct {
	ID               string             `json:"id"`
	PreviewID        string             `json:"preview_id"`
	ActorID          int64              `json:"actor_id"`
	RuleVersion      string             `json:"rule_version"`
	Status           string             `json:"status"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
	BudgetUSD        *float64           `json:"budget_usd"`
	EstimatedCostUSD float64            `json:"estimated_cost_usd"`
	ReservedCostUSD  float64            `json:"reserved_cost_usd"`
	SpentCostUSD     float64            `json:"spent_cost_usd"`
	Total            int                `json:"total"`
	Completed        int                `json:"completed"`
	StaleConfig      bool               `json:"stale_config"`
	Samples          []AcceptanceSample `json:"samples,omitempty"`
}

type ChannelAcceptanceExecutor interface {
	Execute(context.Context, AcceptanceSample) (*AcceptanceResult, error)
}

type ChannelAcceptanceRepository interface {
	SavePreview(context.Context, *AcceptancePreview) error
	GetPreview(context.Context, string, int64) (*AcceptancePreview, error)
	FindRunByPreview(context.Context, string, int64) (*AcceptanceRun, error)
	CreateRun(context.Context, *AcceptancePreview) (*AcceptanceRun, error)
	GetRun(context.Context, string) (*AcceptanceRun, error)
	ListRuns(context.Context, []int64, int) ([]AcceptanceRun, error)
	CancelRun(context.Context, string) error
	ClaimNext(context.Context, string, time.Time) (*AcceptanceSample, error)
	CompleteSample(context.Context, string, string, *AcceptanceResult) error
	RecoverExpired(context.Context, time.Time) error
	Cleanup(context.Context, time.Time) error
}
