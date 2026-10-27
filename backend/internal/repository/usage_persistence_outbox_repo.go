package repository

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	usagePersistencePayloadVersion       = 2
	usagePersistencePayloadLegacyVersion = 1
)
const usagePersistencePayloadMaxBytes = 256 * 1024

// This DTO deliberately enumerates database insert fields. Never marshal the
// service domain object: it includes credential-bearing relations and hides
// UpstreamOutputTokens from its ordinary JSON representation.
type usagePersistencePayload struct {
	Version int                   `json:"version"`
	Usage   usagePersistenceUsage `json:"usage"`
}
type usagePersistenceUsage struct {
	UserID                    int64                    `json:"user_id"`
	APIKeyID                  int64                    `json:"api_key_id"`
	AccountID                 int64                    `json:"account_id"`
	RequestID                 string                   `json:"request_id"`
	Model                     string                   `json:"model"`
	RequestedModel            string                   `json:"requested_model"`
	UpstreamModel             *string                  `json:"upstream_model"`
	UpstreamResponseModel     *string                  `json:"upstream_response_model"`
	UpstreamModelMismatch     *bool                    `json:"upstream_model_mismatch"`
	GroupID                   *int64                   `json:"group_id"`
	SourceGroupID             *int64                   `json:"source_group_id"`
	CustomGroupID             *int64                   `json:"custom_group_id"`
	SubscriptionID            *int64                   `json:"subscription_id"`
	InputTokens               int                      `json:"input_tokens"`
	OutputTokens              int                      `json:"output_tokens"`
	InputTokensSource         service.UsageTokenSource `json:"input_tokens_source,omitempty"`
	OutputTokensSource        service.UsageTokenSource `json:"output_tokens_source,omitempty"`
	UpstreamOutputTokens      int                      `json:"upstream_output_tokens"`
	CacheCreationTokens       int                      `json:"cache_creation_tokens"`
	CacheReadTokens           int                      `json:"cache_read_tokens"`
	CacheCreation5mTokens     int                      `json:"cache_creation_5m_tokens"`
	CacheCreation1hTokens     int                      `json:"cache_creation_1h_tokens"`
	ImageOutputTokens         int                      `json:"image_output_tokens"`
	ImageOutputCost           float64                  `json:"image_output_cost"`
	ImageInputTokens          int                      `json:"image_input_tokens"`
	ImageInputCost            float64                  `json:"image_input_cost"`
	InputCost                 float64                  `json:"input_cost"`
	OutputCost                float64                  `json:"output_cost"`
	CacheCreationCost         float64                  `json:"cache_creation_cost"`
	CacheReadCost             float64                  `json:"cache_read_cost"`
	TotalCost                 float64                  `json:"total_cost"`
	ActualCost                float64                  `json:"actual_cost"`
	ThresholdExemptCost       float64                  `json:"threshold_exempt_cost"`
	RateMultiplier            float64                  `json:"rate_multiplier"`
	AccountRateMultiplier     *float64                 `json:"account_rate_multiplier"`
	BillingType               int8                     `json:"billing_type"`
	RequestType               service.RequestType      `json:"request_type"`
	Stream                    bool                     `json:"stream"`
	OpenAIWSMode              bool                     `json:"openai_ws_mode"`
	DurationMs                *int                     `json:"duration_ms"`
	FirstTokenMs              *int                     `json:"first_token_ms"`
	UserAgent                 *string                  `json:"user_agent"`
	IPAddress                 *string                  `json:"ip_address"`
	ImageCount                int                      `json:"image_count"`
	ImageSize                 *string                  `json:"image_size"`
	ImageInputSize            *string                  `json:"image_input_size"`
	ImageOutputSize           *string                  `json:"image_output_size"`
	ImageSizeSource           *string                  `json:"image_size_source"`
	ImageSizeBreakdown        map[string]int           `json:"image_size_breakdown"`
	VideoCount                int                      `json:"video_count"`
	VideoResolution           *string                  `json:"video_resolution"`
	VideoDurationSeconds      *int                     `json:"video_duration_seconds"`
	ServiceTier               *string                  `json:"service_tier"`
	ReasoningEffort           *string                  `json:"reasoning_effort"`
	RequestedReasoningEffort  *string                  `json:"requested_reasoning_effort"`
	InboundEndpoint           *string                  `json:"inbound_endpoint"`
	UpstreamEndpoint          *string                  `json:"upstream_endpoint"`
	CacheTTLOverridden        bool                     `json:"cache_ttl_overridden"`
	LongContextBillingApplied bool                     `json:"long_context_billing_applied"`
	ChannelID                 *int64                   `json:"channel_id"`
	ModelMappingChain         *string                  `json:"model_mapping_chain"`
	BillingTier               *string                  `json:"billing_tier"`
	BillingMode               *string                  `json:"billing_mode"`
	AccountStatsCost          *float64                 `json:"account_stats_cost"`
	UpstreamRequestID         *string                  `json:"upstream_request_id"`
	SessionID                 *string                  `json:"session_id"`
	NativeCompactionV2        bool                     `json:"native_compaction_v2"`
	CreatedAt                 time.Time                `json:"created_at"`
	AntiTruncationRounds      int                      `json:"anti_truncation_rounds"`
	Outcome                   *usagePersistenceOutcome `json:"outcome,omitempty"`
}

type usagePersistenceOutcome struct {
	HTTPStatus        int                       `json:"http_status"`
	UpstreamStatus    int                       `json:"upstream_status"`
	HasText           bool                      `json:"has_text"`
	HasToolCall       bool                      `json:"has_tool_call"`
	HasReasoning      bool                      `json:"has_reasoning"`
	HasMedia          bool                      `json:"has_media"`
	OutputBytes       int64                     `json:"output_bytes"`
	EventCount        int                       `json:"event_count"`
	StreamCompleted   bool                      `json:"stream_completed"`
	FinishReason      string                    `json:"finish_reason"`
	DisconnectSource  service.DisconnectSource  `json:"disconnect_source"`
	UpstreamErrorKind service.UpstreamErrorKind `json:"upstream_error_kind"`
	CollectorVersion  int                       `json:"collector_version"`
}

func encodeUsagePersistencePayload(cmd *service.UsageBillingCommand, thresholdExemptCost float64) ([]byte, error) {
	log := cmd.UsageLog
	if log == nil {
		return nil, nil
	}
	if cmd.RequestID == "" || log.APIKeyID <= 0 || log.UserID <= 0 || log.AccountID <= 0 || strings.TrimSpace(log.RequestID) != cmd.RequestID || log.APIKeyID != cmd.APIKeyID || log.UserID != cmd.UserID || log.AccountID != cmd.AccountID {
		return nil, errors.New("usage persistence snapshot identity mismatch")
	}
	payload := usagePersistencePayload{Version: usagePersistencePayloadVersion, Usage: usagePersistenceUsage{
		UserID:                    log.UserID,
		APIKeyID:                  log.APIKeyID,
		AccountID:                 log.AccountID,
		RequestID:                 log.RequestID,
		Model:                     log.Model,
		RequestedModel:            log.RequestedModel,
		UpstreamModel:             log.UpstreamModel,
		UpstreamResponseModel:     log.UpstreamResponseModel,
		UpstreamModelMismatch:     log.UpstreamModelMismatch,
		GroupID:                   log.GroupID,
		SourceGroupID:             log.SourceGroupID,
		CustomGroupID:             log.CustomGroupID,
		SubscriptionID:            log.SubscriptionID,
		InputTokens:               log.InputTokens,
		OutputTokens:              log.OutputTokens,
		InputTokensSource:         log.InputTokensSource,
		OutputTokensSource:        log.OutputTokensSource,
		UpstreamOutputTokens:      log.UpstreamOutputTokens,
		CacheCreationTokens:       log.CacheCreationTokens,
		CacheReadTokens:           log.CacheReadTokens,
		CacheCreation5mTokens:     log.CacheCreation5mTokens,
		CacheCreation1hTokens:     log.CacheCreation1hTokens,
		ImageOutputTokens:         log.ImageOutputTokens,
		ImageOutputCost:           log.ImageOutputCost,
		ImageInputTokens:          log.ImageInputTokens,
		ImageInputCost:            log.ImageInputCost,
		InputCost:                 log.InputCost,
		OutputCost:                log.OutputCost,
		CacheCreationCost:         log.CacheCreationCost,
		CacheReadCost:             log.CacheReadCost,
		TotalCost:                 log.TotalCost,
		ActualCost:                log.ActualCost,
		ThresholdExemptCost:       log.ThresholdExemptCost,
		RateMultiplier:            log.RateMultiplier,
		AccountRateMultiplier:     log.AccountRateMultiplier,
		BillingType:               log.BillingType,
		RequestType:               log.RequestType,
		Stream:                    log.Stream,
		OpenAIWSMode:              log.OpenAIWSMode,
		DurationMs:                log.DurationMs,
		FirstTokenMs:              log.FirstTokenMs,
		UserAgent:                 log.UserAgent,
		IPAddress:                 log.IPAddress,
		ImageCount:                log.ImageCount,
		ImageSize:                 log.ImageSize,
		ImageInputSize:            log.ImageInputSize,
		ImageOutputSize:           log.ImageOutputSize,
		ImageSizeSource:           log.ImageSizeSource,
		ImageSizeBreakdown:        log.ImageSizeBreakdown,
		VideoCount:                log.VideoCount,
		VideoResolution:           log.VideoResolution,
		VideoDurationSeconds:      log.VideoDurationSeconds,
		ServiceTier:               log.ServiceTier,
		ReasoningEffort:           log.ReasoningEffort,
		RequestedReasoningEffort:  log.RequestedReasoningEffort,
		InboundEndpoint:           log.InboundEndpoint,
		UpstreamEndpoint:          log.UpstreamEndpoint,
		CacheTTLOverridden:        log.CacheTTLOverridden,
		LongContextBillingApplied: log.LongContextBillingApplied,
		ChannelID:                 log.ChannelID,
		ModelMappingChain:         log.ModelMappingChain,
		BillingTier:               log.BillingTier,
		BillingMode:               log.BillingMode,
		AccountStatsCost:          log.AccountStatsCost,
		UpstreamRequestID:         log.UpstreamRequestID,
		SessionID:                 log.SessionID,
		NativeCompactionV2:        log.NativeCompactionV2,
		CreatedAt:                 log.CreatedAt,
		AntiTruncationRounds:      log.AntiTruncationRounds,
	}}
	payload.Usage.RequestID = cmd.RequestID
	payload.Usage.ThresholdExemptCost = thresholdExemptCost
	payload.Usage.ActualCost = service.QuantizeUsageBillingAmount(log.ActualCost)
	if payload.Usage.CreatedAt.IsZero() {
		payload.Usage.CreatedAt = time.Now().UTC()
	}
	if log.Outcome != nil {
		payload.Usage.Outcome = &usagePersistenceOutcome{
			HTTPStatus:        log.Outcome.HTTPStatus,
			UpstreamStatus:    log.Outcome.UpstreamStatus,
			HasText:           log.Outcome.HasText,
			HasToolCall:       log.Outcome.HasToolCall,
			HasReasoning:      log.Outcome.HasReasoning,
			HasMedia:          log.Outcome.HasMedia,
			OutputBytes:       log.Outcome.OutputBytes,
			EventCount:        log.Outcome.EventCount,
			StreamCompleted:   log.Outcome.StreamCompleted,
			FinishReason:      log.Outcome.FinishReason,
			DisconnectSource:  log.Outcome.DisconnectSource,
			UpstreamErrorKind: log.Outcome.UpstreamErrorKind,
			CollectorVersion:  log.Outcome.CollectorVersion,
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.New("usage persistence snapshot cannot be encoded")
	}
	if len(raw) > usagePersistencePayloadMaxBytes {
		return nil, errors.New("usage persistence snapshot exceeds size limit")
	}
	return raw, nil
}

func decodeUsagePersistencePayload(raw []byte, requestID string, apiKeyID int64) (*service.UsageLog, error) {
	// JSONB adds formatting spaces on read. Bound its representation first,
	// then enforce the original admission bound against compact JSON content.
	if len(raw) == 0 || len(raw) > 2*usagePersistencePayloadMaxBytes {
		return nil, errors.New("invalid usage persistence snapshot size")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return nil, errors.New("invalid usage persistence snapshot")
	}
	if compact.Len() > usagePersistencePayloadMaxBytes {
		return nil, errors.New("invalid usage persistence snapshot size")
	}
	var payload usagePersistencePayload
	decoder := json.NewDecoder(&compact)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return nil, errors.New("invalid usage persistence snapshot")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("invalid trailing usage persistence snapshot")
	}
	if payload.Version != usagePersistencePayloadLegacyVersion && payload.Version != usagePersistencePayloadVersion {
		return nil, errors.New("unsupported usage persistence snapshot version")
	}
	usage := payload.Usage
	if payload.Version == usagePersistencePayloadLegacyVersion {
		// Keep the deployed v1 contract strict: neither provenance key was part
		// of it, including an explicitly empty or null value. Read old snapshots
		// without inferring sources from their numeric counts.
		var legacy struct {
			Usage map[string]json.RawMessage `json:"usage"`
		}
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return nil, errors.New("invalid legacy usage persistence snapshot")
		}
		if _, present := legacy.Usage["input_tokens_source"]; present {
			return nil, errors.New("v1 usage persistence snapshot contains v2 metadata")
		}
		if _, present := legacy.Usage["output_tokens_source"]; present {
			return nil, errors.New("v1 usage persistence snapshot contains v2 metadata")
		}
		usage.InputTokensSource = service.UsageTokenSourceUnknown
		usage.OutputTokensSource = service.UsageTokenSourceUnknown
	}
	if usage.RequestID != requestID || usage.APIKeyID != apiKeyID || requestID == "" || apiKeyID <= 0 || usage.UserID <= 0 || usage.AccountID <= 0 || usage.CreatedAt.IsZero() {
		return nil, errors.New("invalid usage persistence snapshot identity or version")
	}
	log := &service.UsageLog{
		UserID:                    usage.UserID,
		APIKeyID:                  usage.APIKeyID,
		AccountID:                 usage.AccountID,
		RequestID:                 usage.RequestID,
		Model:                     usage.Model,
		RequestedModel:            usage.RequestedModel,
		UpstreamModel:             usage.UpstreamModel,
		UpstreamResponseModel:     usage.UpstreamResponseModel,
		UpstreamModelMismatch:     usage.UpstreamModelMismatch,
		GroupID:                   usage.GroupID,
		SourceGroupID:             usage.SourceGroupID,
		CustomGroupID:             usage.CustomGroupID,
		SubscriptionID:            usage.SubscriptionID,
		InputTokens:               usage.InputTokens,
		OutputTokens:              usage.OutputTokens,
		InputTokensSource:         usage.InputTokensSource,
		OutputTokensSource:        usage.OutputTokensSource,
		UpstreamOutputTokens:      usage.UpstreamOutputTokens,
		CacheCreationTokens:       usage.CacheCreationTokens,
		CacheReadTokens:           usage.CacheReadTokens,
		CacheCreation5mTokens:     usage.CacheCreation5mTokens,
		CacheCreation1hTokens:     usage.CacheCreation1hTokens,
		ImageOutputTokens:         usage.ImageOutputTokens,
		ImageOutputCost:           usage.ImageOutputCost,
		ImageInputTokens:          usage.ImageInputTokens,
		ImageInputCost:            usage.ImageInputCost,
		InputCost:                 usage.InputCost,
		OutputCost:                usage.OutputCost,
		CacheCreationCost:         usage.CacheCreationCost,
		CacheReadCost:             usage.CacheReadCost,
		TotalCost:                 usage.TotalCost,
		ActualCost:                usage.ActualCost,
		ThresholdExemptCost:       usage.ThresholdExemptCost,
		RateMultiplier:            usage.RateMultiplier,
		AccountRateMultiplier:     usage.AccountRateMultiplier,
		BillingType:               usage.BillingType,
		RequestType:               usage.RequestType,
		Stream:                    usage.Stream,
		OpenAIWSMode:              usage.OpenAIWSMode,
		DurationMs:                usage.DurationMs,
		FirstTokenMs:              usage.FirstTokenMs,
		UserAgent:                 usage.UserAgent,
		IPAddress:                 usage.IPAddress,
		ImageCount:                usage.ImageCount,
		ImageSize:                 usage.ImageSize,
		ImageInputSize:            usage.ImageInputSize,
		ImageOutputSize:           usage.ImageOutputSize,
		ImageSizeSource:           usage.ImageSizeSource,
		ImageSizeBreakdown:        usage.ImageSizeBreakdown,
		VideoCount:                usage.VideoCount,
		VideoResolution:           usage.VideoResolution,
		VideoDurationSeconds:      usage.VideoDurationSeconds,
		ServiceTier:               usage.ServiceTier,
		ReasoningEffort:           usage.ReasoningEffort,
		RequestedReasoningEffort:  usage.RequestedReasoningEffort,
		InboundEndpoint:           usage.InboundEndpoint,
		UpstreamEndpoint:          usage.UpstreamEndpoint,
		CacheTTLOverridden:        usage.CacheTTLOverridden,
		LongContextBillingApplied: usage.LongContextBillingApplied,
		ChannelID:                 usage.ChannelID,
		ModelMappingChain:         usage.ModelMappingChain,
		BillingTier:               usage.BillingTier,
		BillingMode:               usage.BillingMode,
		AccountStatsCost:          usage.AccountStatsCost,
		UpstreamRequestID:         usage.UpstreamRequestID,
		SessionID:                 usage.SessionID,
		NativeCompactionV2:        usage.NativeCompactionV2,
		CreatedAt:                 usage.CreatedAt,
		AntiTruncationRounds:      usage.AntiTruncationRounds,
	}
	if usage.Outcome != nil {
		log.Outcome = &service.ResponseOutcome{
			HTTPStatus:        usage.Outcome.HTTPStatus,
			UpstreamStatus:    usage.Outcome.UpstreamStatus,
			HasText:           usage.Outcome.HasText,
			HasToolCall:       usage.Outcome.HasToolCall,
			HasReasoning:      usage.Outcome.HasReasoning,
			HasMedia:          usage.Outcome.HasMedia,
			OutputBytes:       usage.Outcome.OutputBytes,
			EventCount:        usage.Outcome.EventCount,
			StreamCompleted:   usage.Outcome.StreamCompleted,
			FinishReason:      usage.Outcome.FinishReason,
			DisconnectSource:  usage.Outcome.DisconnectSource,
			UpstreamErrorKind: usage.Outcome.UpstreamErrorKind,
			CollectorVersion:  usage.Outcome.CollectorVersion,
		}
	}
	return log, nil
}

func enqueueUsagePersistence(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, thresholdExemptCost float64) error {
	if cmd.UsageLog == nil {
		return nil
	}
	payload, err := encodeUsagePersistencePayload(cmd, thresholdExemptCost)
	if err != nil {
		return err
	}
	// A new billing claim must own exactly one new snapshot; conflicts fail the
	// entire charge instead of silently accepting an unrelated old payload.
	_, err = tx.ExecContext(ctx, `INSERT INTO usage_persistence_outbox (request_id,api_key_id,payload) VALUES ($1,$2,$3::jsonb)`, cmd.RequestID, cmd.APIKeyID, string(payload))
	return err
}

type usagePersistenceOutboxRepository struct{ db *sql.DB }

func NewUsagePersistenceOutboxRepository(_ *dbent.Client, db *sql.DB) service.UsagePersistenceOutboxRepository {
	return &usagePersistenceOutboxRepository{db: db}
}

func validateUsagePersistenceWorker(workerID string) error {
	if workerID != strings.TrimSpace(workerID) || workerID == "" || len(workerID) > 128 {
		return errors.New("invalid usage persistence worker identity")
	}
	return nil
}

func (r *usagePersistenceOutboxRepository) Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]service.UsagePersistenceEvent, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil usage persistence database")
	}
	if err := validateUsagePersistenceWorker(workerID); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 8
	}
	if limit > 100 {
		limit = 100
	}
	leaseSeconds := int64(lease / time.Second)
	if leaseSeconds < 1 {
		leaseSeconds = 60
	}
	rows, err := r.db.QueryContext(ctx, `
 WITH candidates AS (
  SELECT id FROM usage_persistence_outbox
  WHERE available_at<=NOW() AND (claimed_at IS NULL OR claimed_at<NOW()-($3*INTERVAL '1 second'))
  ORDER BY available_at,id LIMIT $2 FOR UPDATE SKIP LOCKED
 )
 UPDATE usage_persistence_outbox AS o SET claimed_at=NOW(),claimed_by=$1
 FROM candidates AS c WHERE o.id=c.id
 RETURNING o.id,o.attempts,o.created_at`, workerID, limit, leaseSeconds)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	events := make([]service.UsagePersistenceEvent, 0, limit)
	for rows.Next() {
		var event service.UsagePersistenceEvent
		if err := rows.Scan(&event.ID, &event.Attempts, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (r *usagePersistenceOutboxRepository) Deliver(ctx context.Context, id int64, workerID string) error {
	if r == nil || r.db == nil {
		return errors.New("nil usage persistence database")
	}
	if err := validateUsagePersistenceWorker(workerID); err != nil {
		return err
	}
	if id <= 0 {
		return errors.New("invalid usage persistence event identity")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var requestID string
	var apiKeyID int64
	var payload []byte
	var owner sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT request_id,api_key_id,payload,claimed_by FROM usage_persistence_outbox WHERE id=$1 FOR UPDATE`, id).Scan(&requestID, &apiKeyID, &payload, &owner)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !owner.Valid || owner.String != workerID {
		return errors.New("usage persistence claim no longer owned")
	}
	log, err := decodeUsagePersistencePayload(payload, requestID, apiKeyID)
	if err != nil {
		return err
	}
	// Both repositories use this exact SQL transaction; the worker never calls
	// billing or performs another wallet, quota, or dedup mutation.
	usageRepo := &usageLogRepository{sql: tx}
	inserted, err := usageRepo.createSingle(ctx, tx, log)
	if err != nil {
		return err
	}
	if inserted {
		if err := usageRepo.upsertResponseOutcome(ctx, log); err != nil {
			return err
		}
	}
	// Existing history is immutable. New-path outcome and usage are atomic, so
	// a conflicting old row must not be rewritten using later retry evidence.
	result, err := tx.ExecContext(ctx, `DELETE FROM usage_persistence_outbox WHERE id=$1 AND claimed_by=$2`, id, workerID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("usage persistence claim no longer owned")
	}
	return tx.Commit()
}

func (r *usagePersistenceOutboxRepository) RetryClaimed(ctx context.Context, id int64, workerID string, availableAt time.Time, lastError string) error {
	if r == nil || r.db == nil {
		return errors.New("nil usage persistence database")
	}
	if err := validateUsagePersistenceWorker(workerID); err != nil {
		return err
	}
	if id <= 0 || availableAt.IsZero() {
		return errors.New("invalid usage persistence retry")
	}
	// Only fixed classifications cross the persistence boundary; SQL errors may
	// contain customer field values or operational credentials.
	switch lastError {
	case "usage_persistence_delivery_timeout", "usage_persistence_payload_invalid", "usage_persistence_delivery_failed":
	default:
		lastError = "usage_persistence_delivery_failed"
	}
	result, err := r.db.ExecContext(ctx, `
 UPDATE usage_persistence_outbox
 SET attempts=CASE WHEN attempts<2147483647 THEN attempts+1 ELSE attempts END,
 available_at=$3,last_error=$4,claimed_at=NULL,claimed_by=NULL
 WHERE id=$1 AND claimed_by=$2`, id, workerID, availableAt, lastError)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("usage persistence claim no longer owned")
	}
	return nil
}
