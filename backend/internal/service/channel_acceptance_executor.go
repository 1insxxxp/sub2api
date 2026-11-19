package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
)

type channelAcceptanceAuditKey struct{}

func isChannelAcceptanceAudit(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(channelAcceptanceAuditKey{}).(bool)
	return v
}

// AccountAcceptanceExecutor calls production adapters, never public handlers.
// No customer identity, ledger, admission, scheduler or billing service is used.
type AccountAcceptanceExecutor struct {
	accounts    AccountRepository
	groups      GroupRepository
	gemini      *GeminiMessagesCompatService
	antigravity *AntigravityGatewayService
	openai      *OpenAIGatewayService
	anthropic   *GatewayService
}

func NewAccountAcceptanceExecutor(accounts AccountRepository, groups GroupRepository, gemini *GeminiMessagesCompatService, antigravity *AntigravityGatewayService, openai *OpenAIGatewayService, anthropic *GatewayService) *AccountAcceptanceExecutor {
	return &AccountAcceptanceExecutor{accounts: accounts, groups: groups, gemini: gemini, antigravity: antigravity, openai: openai, anthropic: anthropic}
}

var _ ChannelAcceptanceExecutor = (*AccountAcceptanceExecutor)(nil)

func (e *AccountAcceptanceExecutor) Execute(ctx context.Context, sample AcceptanceSample) (*AcceptanceResult, error) {
	started := time.Now()
	a, err := e.accounts.GetByID(ctx, sample.Snapshot.AccountID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, ErrAcceptanceNotFound
	}
	var group *Group
	if sample.Snapshot.GroupID != nil {
		if e.groups == nil {
			return nil, ErrAcceptanceInvalid
		}
		group, err = e.groups.GetByID(ctx, *sample.Snapshot.GroupID)
		if err != nil {
			return nil, err
		}
	}
	if AcceptanceAccountFingerprint(a, group) != sample.Snapshot.Fingerprint {
		return nil, ErrAcceptanceStale
	}
	// Deep copy credentials, settings and embedded maps: a production forwarder
	// may update runtime account fields even when repository writes are suppressed.
	cloneBytes, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	var copyAccount Account
	if err = json.Unmarshal(cloneBytes, &copyAccount); err != nil {
		return nil, err
	}
	a = &copyAccount
	sample.Snapshot.Platform = a.Platform
	result := &AcceptanceResult{Status: "inconclusive", Source: AcceptanceSource{Kind: "unverified", Channel: "unknown", RequestedModel: sample.Snapshot.Model, UpstreamModel: sample.Snapshot.UpstreamModel}, EffectiveParameters: map[string]any{"requested_max_output_tokens": sample.MaxOutputTokens, "fixed_account": true, "automatic_continuation": false}}
	if sample.Protocol != "count_tokens" && acceptanceThinkingConfig(sample.Snapshot.UpstreamModel) != nil {
		result.EffectiveParameters["requested_reasoning_effort"] = "low"
	}
	unsupported := func(reason string) (*AcceptanceResult, error) {
		result.Status = "unsupported"
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "unsupported", Level: "info", Message: reason})
		zero := float64(0)
		result.CostUSD = &zero
		return result, nil
	}
	if sample.Category == "audio" && (sample.Protocol == "messages" || sample.Protocol == "responses") {
		return unsupported("该协议没有音频验收请求体")
	}
	if a.IsShadow() || a.IsOpenAIAgentIdentity() {
		return unsupported("此账号需借用父账号凭据或动态身份，无法独立验收")
	}
	if a.Platform == PlatformOpenAI && sample.Protocol == "messages" && group != nil && !group.AllowMessagesDispatch {
		return unsupported("当前分组未启用 Messages 转发")
	}
	if sample.Protocol == "count_tokens" {
		if !(a.Platform == PlatformGemini && (a.Type == AccountTypeServiceAccount || a.Type == AccountTypeAPIKey) || a.Platform == PlatformAnthropic && a.Type == AccountTypeAPIKey) {
			return unsupported("没有相同来源的原生计数适配器")
		}
	} else if !slices.Contains(acceptanceProtocols(a.Platform, a.Type, sample.Snapshot.UpstreamModel), sample.Protocol) {
		return unsupported("此账号类型和模型没有对应协议的生产转发入口")
	}
	if sample.Protocol == "systemone" && (sample.Stream || sample.Category == "audio" || sample.Category == "image" || sample.Category == "tools") {
		return unsupported("System One 仅支持非流式结构化文本样本")
	}
	if a.Platform == PlatformAntigravity && (a.Type == AccountTypeUpstream && sample.Protocol != "messages" || a.Type == AccountTypeAPIKey && sample.Protocol != "gemini" && strings.TrimSpace(a.GetCredential("base_url")) == "") {
		return unsupported("反重力中转账号没有该协议的固定上游地址")
	}
	if a.Type == AccountTypeOAuth {
		if expiry := a.GetCredentialAsTime("expires_at"); expiry != nil && !expiry.After(time.Now().Add(time.Minute)) {
			return unsupported("OAuth 凭据即将到期，请先通过正常账号刷新后验收；验收不触发状态写入")
		}
	}
	timeout := sample.TimeoutSeconds
	if timeout <= 0 || timeout > 360 {
		timeout = 180
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	ctx = context.WithValue(WithModelProbeExecution(ctx), channelAcceptanceAuditKey{}, true)
	ctx = context.WithValue(ctx, accountContinuationManagedKey{}, true)
	// Production DI always supplies OpenAIGatewayService, which owns the same
	// shared account concurrency service used by every public gateway. Acquire
	// only a free slot; audits do not queue ahead of live customer traffic.
	if e.openai != nil && e.openai.concurrencyService != nil {
		slot, slotErr := e.openai.concurrencyService.AcquireAccountSlot(ctx, a.ID, a.Concurrency)
		if slotErr != nil || slot == nil || !slot.Acquired {
			zero := float64(0)
			result.CostUSD = &zero
			result.Findings = append(result.Findings, AcceptanceFinding{Code: "account_busy", Level: "info", Message: "账号并发已满或无法确认空闲槽，本样本未发送，请稍后重试"})
			return result, nil
		}
		if slot.ReleaseFunc != nil {
			defer slot.ReleaseFunc()
		}
	}
	body, err := acceptanceRequestBody(sample)
	if err != nil {
		return nil, err
	}
	client := newAcceptanceEvidenceParser()
	writer := &acceptanceResponseWriter{header: make(http.Header), parser: client, started: started}
	c, _ := gin.CreateTestContext(writer)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")
	if group != nil {
		key := &APIKey{Group: group, GroupID: &group.ID}
		c.Set("api_key", key)
		ctx = context.WithValue(ctx, ctxkey.APIKey, key)
		ctx = context.WithValue(ctx, ctxkey.Group, group)
		c.Request = c.Request.WithContext(ctx)
	}
	action := "generateContent"
	if sample.Stream {
		action = "streamGenerateContent"
	}
	if sample.Protocol == "count_tokens" {
		action = "countTokens"
	}
	switch sample.Protocol {
	case "systemone":
		c.Request.URL.Path = "/v1/systemone"
	case "responses":
		c.Request.URL.Path = "/v1/responses"
	case "messages":
		c.Request.URL.Path = "/v1/messages"
	case "gemini", "count_tokens":
		c.Request.URL.Path = "/v1beta/models/" + sample.Snapshot.Model + ":" + action
		if sample.Protocol == "count_tokens" && a.Platform == PlatformAnthropic {
			c.Request.URL.Path = "/v1/messages/count_tokens"
		}
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/messages") {
		c.Request.Header.Set("anthropic-version", "2023-06-01")
	}
	c.Request.RequestURI = c.Request.URL.RequestURI()
	upstream := &acceptanceHTTPUpstream{accountID: a.ID, parser: newAcceptanceEvidenceParser(), result: result, sample: sample}
	repo := &acceptanceReadOnlyAccounts{AccountRepository: e.accounts}
	var fr *ForwardResult
	var ofr *OpenAIForwardResult
	switch {
	case a.Platform == PlatformAntigravity && a.Type != AccountTypeAPIKey:
		if e.antigravity == nil || a.Type == AccountTypeOAuth && e.antigravity.tokenProvider == nil {
			return unsupported("反重力转发服务未配置")
		}
		s := e.auditAntigravity(repo, upstream)
		switch sample.Protocol {
		case "chat":
			fr, err = s.ForwardAsChatCompletions(ctx, c, a, body, nil)
		case "messages":
			fr, err = s.Forward(ctx, c, a, body, false)
		case "responses":
			fr, err = s.ForwardAsResponses(ctx, c, a, body, nil)
		case "gemini":
			fr, err = s.ForwardGemini(ctx, c, a, sample.Snapshot.Model, action, sample.Stream, body, false)
		default:
			return unsupported("反重力不支持此协议")
		}
	case domain.UsesOpenAIGateway(a.Platform):
		if e.openai == nil {
			return unsupported("OpenAI 兼容转发服务未配置")
		}
		upstream.base = e.openai.httpUpstream
		cfg := acceptanceConfig(e.openai.cfg)
		// Do not shallow-copy this service: it owns mutexes, sync.Once and maps.
		s := &OpenAIGatewayService{accountRepo: repo, httpUpstream: upstream, cfg: cfg, responseHeaderFilter: e.openai.responseHeaderFilter, codexDetector: e.openai.codexDetector, toolCorrector: NewCodexToolCorrector(), resolver: e.openai.resolver, rateLimitService: &RateLimitService{accountRepo: repo, cfg: cfg}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg)}
		switch sample.Protocol {
		case "chat":
			ofr, err = s.ForwardAsChatCompletions(ctx, c, a, body, "", "")
		case "responses":
			ofr, err = s.Forward(ctx, c, a, body)
		case "messages":
			ofr, err = s.ForwardAsAnthropic(ctx, c, a, body, "", "")
		default:
			return unsupported("OpenAI 网关不支持此协议")
		}
	case a.Platform == PlatformGemini || a.Platform == PlatformAntigravity && sample.Protocol == "gemini":
		if e.gemini == nil {
			return unsupported("Gemini 转发服务未配置")
		}
		s := e.auditGemini(repo, upstream)
		switch sample.Protocol {
		case "chat":
			fr, err = s.ForwardAsChatCompletions(ctx, c, a, body)
		case "messages":
			fr, err = s.Forward(ctx, c, a, body)
		case "count_tokens", "gemini":
			fr, err = s.ForwardNative(ctx, c, a, sample.Snapshot.Model, action, sample.Stream, body)
		default:
			return unsupported("Gemini 网关不支持此协议")
		}
	case a.Platform == PlatformAnthropic || a.Platform == PlatformTypeSafe || a.Platform == PlatformAntigravity && a.Type == AccountTypeAPIKey:
		if e.anthropic == nil {
			return unsupported("Anthropic 转发服务未配置")
		}
		upstream.base = e.anthropic.httpUpstream
		cfg := acceptanceConfig(e.anthropic.cfg)
		s := &GatewayService{cfg: cfg, accountRepo: repo, httpUpstream: upstream, responseHeaderFilter: e.anthropic.responseHeaderFilter, tlsFPProfileService: e.anthropic.tlsFPProfileService, settingService: e.anthropic.settingService, rateLimitService: &RateLimitService{accountRepo: repo, cfg: cfg}}
		if a.Type == AccountTypeServiceAccount && e.anthropic.claudeTokenProvider != nil {
			s.claudeTokenProvider = &ClaudeTokenProvider{accountRepo: repo, tokenCache: acceptanceTokenCache(e.anthropic.claudeTokenProvider.tokenCache)}
		}
		if sample.Protocol == "systemone" {
			if a.Platform != PlatformTypeSafe {
				return unsupported("仅 TypeSafe 支持 System One")
			}
			var response *SystemOneForwardResult
			response, err = s.ForwardSystemOne(ctx, c, a, body)
			if response != nil {
				fr = &response.ForwardResult
				c.Data(response.StatusCode, response.ContentType, response.Body)
			}
			break
		}
		parseProtocol := PlatformAnthropic
		switch sample.Protocol {
		case "chat":
			parseProtocol = "chat_completions"
		case "responses":
			parseProtocol = "responses"
		}
		parsed, parseErr := ParseGatewayRequest(NewRequestBodyRef(body), parseProtocol)
		if parseErr != nil {
			return nil, parseErr
		}
		parsed.GroupID = sample.Snapshot.GroupID
		switch sample.Protocol {
		case "chat":
			fr, err = s.ForwardAsChatCompletions(ctx, c, a, body, parsed)
		case "responses":
			fr, err = s.ForwardAsResponses(ctx, c, a, body, parsed)
		case "messages":
			fr, err = s.Forward(ctx, c, a, parsed)
		case "count_tokens":
			err = s.ForwardCountTokens(ctx, c, a, parsed)
		default:
			return unsupported("Anthropic 网关不支持此协议")
		}
	default:
		return unsupported("未知账号平台")
	}
	client.Finish()
	upstream.parser.Finish()
	client = client.snapshot()
	raw := upstream.parser.snapshot()
	result.DurationMS = time.Since(started).Milliseconds()
	result.HTTPStatus = writer.status
	if upstream.status != 0 {
		result.HTTPStatus = upstream.status
	}
	result.RawUsage = raw.usage
	result.UpstreamUsageFinal = raw.usageFinal
	result.ClientUsage = client.usage
	result.ClientUsage.Source = "client_wire:" + client.usage.Source
	result.UsageFrames = raw.frames
	result.Source.ResponseModel = acceptanceRedactExcerpt(raw.responseModel, a)
	result.Source.Adapter = a.Platform + "/" + sample.Protocol
	if result.Source.AuthKind == "bearer" {
		if a.Type == AccountTypeOAuth {
			result.Source.AuthKind = "oauth"
		}
		if a.Type == AccountTypeServiceAccount {
			result.Source.AuthKind = "service_account"
		}
	}
	result.FinishReason = client.finishReason
	result.Completed = client.completed
	result.HasText = client.hasText
	result.HasReasoning = client.hasReasoning
	result.HasTool = client.hasTool
	result.OutputExcerpt = acceptanceRedactExcerpt(client.excerpt, a)
	result.FirstTokenMS = writer.firstToken
	result.BillingUsage = acceptanceBillingProjection(fr, ofr, result.RawUsage)
	acceptanceCaptureBillingOrigins(result, fr, ofr)
	if fr != nil {
		if fr.GeminiInputCounting != nil {
			result.EffectiveParameters["input_counting_policy"] = fr.GeminiInputCounting.Policy
		}
		if fr.ReasoningEffort != nil {
			result.EffectiveParameters["resolved_reasoning_effort"] = *fr.ReasoningEffort
		}
	}
	if minimum := acceptanceCacheMinimum(result.Source.UpstreamModel); minimum > 0 {
		result.EffectiveParameters["cache_min_input_tokens"] = minimum
		if result.RawUsage.Input != nil {
			result.EffectiveParameters["cache_input_condition_met"] = *result.RawUsage.Input >= int64(minimum)
		}
	}
	result.EffectiveParameters["account_rate_multiplier"] = a.BillingRateMultiplier()
	if group != nil {
		result.EffectiveParameters["group_rate_multiplier"] = group.RateMultiplier
	}
	if raw.truncated || client.truncated {
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "evidence_truncated", Level: "warning", Message: "单个证据帧超过安全上限，部分字段无法评估"})
	}
	if sample.Protocol == "count_tokens" {
		if raw.hasText || raw.hasReasoning || raw.hasTool || raw.usage.Output != nil {
			result.Status = "anomaly"
			result.Findings = append(result.Findings, AcceptanceFinding{Code: "count_returned_generation", Level: "warning", Message: "计数接口返回了生成内容，不能作为计数基准；本次可能产生生成费用，停止该来源后续计数测试"})
			return result, nil
		}
		zero := float64(0)
		result.CostUSD = &zero
		result.EffectiveParameters["cost_assumption"] = "count_tokens_not_charged"
		if upstream.status >= 200 && upstream.status < 300 && result.RawUsage.Input != nil && result.RawUsage.Source == "upstream_count_tokens" {
			result.Completed = true
			result.Status = "passed"
		} else {
			result.Status = "unsupported"
			result.Findings = append(result.Findings, AcceptanceFinding{Code: "count_not_measured", Level: "info", Message: "未获得同一上游真实计数；本地回退值不作为测量"})
		}
		return result, nil
	}
	if !client.completed {
		code, message := "response_incomplete", "响应未包含完整结束标记，无法确认本次输出已完整返回"
		if sample.Stream {
			code, message = "stream_incomplete", "流式响应未包含完整结束事件，已收到内容和用量仅作为部分证据"
		}
		result.Findings = append(result.Findings, AcceptanceFinding{Code: code, Level: "warning", Message: message})
	}
	if !raw.usageFinal {
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "upstream_usage_not_final", Level: "warning", Message: "未收到上游结束时或结束后的用量；保留已收到统计，不作为最终费用依据"})
	}
	reasoningOnly := !result.HasText && !result.HasTool && (result.HasReasoning || (raw.usage.Thoughts != nil && *raw.usage.Thoughts > 0))
	if reasoningOnly {
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "reasoning_only", Level: "warning", Message: "仅返回思考内容，没有普通正文或工具调用，无法验证本次回答"})
	}
	if err != nil || result.HTTPStatus >= 400 {
		result.Status = "error"
		if errors.Is(ctx.Err(), context.Canceled) {
			result.Status = "cancelled"
		}
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "forward_failed", Level: "error", Message: "固定账号请求未正常完成；没有切号或额外推理重试"})
		return result, nil
	}
	if client.completed && !reasoningOnly && raw.usageFinal {
		result.Status = "passed"
	}
	if result.RawUsage.Output == nil || result.RawUsage.Input == nil {
		result.Status = "inconclusive"
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "usage_missing", Level: "warning", Message: "上游缺少输入或输出用量，无法认定计量正常"})
	}
	if result.RawUsage.Output != nil && *result.RawUsage.Output > 0 && (result.ClientUsage.Output == nil || *result.ClientUsage.Output == 0) {
		result.Status = "anomaly"
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "client_output_usage_lost", Level: "error", Message: "上游存在输出用量，但客户端返回缺失或为零"})
	}
	if result.RawUsage.Output != nil && *result.RawUsage.Output == 0 && (raw.hasText || raw.hasTool) {
		result.Status = "anomaly"
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "upstream_zero_output_with_content", Level: "warning", Message: "上游返回了正文或工具调用，但同时明确报告输出为零；需要复核计量"})
	}
	if result.RawUsage.Source == "upstream_chat_reported" && result.RawUsage.Thoughts == nil {
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "reasoning_breakdown_missing", Level: "info", Message: "Chat 输出总量包含未拆分的思考消耗；不能据此认定思考为零"})
	}
	if acceptanceUsageContradictory(result.RawUsage) {
		result.Status = "anomaly"
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "usage_inconsistent", Level: "warning", Message: "上游用量字段之间存在矛盾，需要复核字段口径，不能据此断言造假"})
	}
	matched := strings.Contains(result.OutputExcerpt, sample.Expected)
	if sample.Category == "image" {
		matched = strings.Contains(strings.ToLower(result.OutputExcerpt), strings.ToLower(sample.Expected))
	}
	if sample.Category == "tools" {
		matched = client.toolMatches(sample.Expected)
	}
	if client.completed && !reasoningOnly && sample.Expected != "" && !matched {
		result.Status = "anomaly"
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "challenge_mismatch", Level: "warning", Message: "回答未包含当前样本的校验值，需要复核是否忽略新输入"})
	}
	if sample.Category == "audio" {
		result.Findings = append(result.Findings, AcceptanceFinding{Code: "audio_fixture_limit", Level: "info", Message: "测试音频为合成单音，仅验证音频输入兼容，不证明语音转写质量"})
	}
	return result, nil
}

func acceptanceConfig(original *config.Config) *config.Config {
	var c config.Config
	if original != nil {
		c = *original
	}
	c.Gateway.MaxSameAccountRetriesGemini = 0
	return &c
}
func (e *AccountAcceptanceExecutor) auditGemini(repo AccountRepository, u *acceptanceHTTPUpstream) *GeminiMessagesCompatService {
	s := *e.gemini
	s.accountRepo = repo
	s.cache = nil
	s.schedulerSnapshot = nil
	s.rateLimitService = nil
	s.settingService = nil
	s.cfg = acceptanceConfig(e.gemini.cfg)
	u.base = e.gemini.httpUpstream
	s.httpUpstream = u
	if e.gemini.tokenProvider != nil {
		s.tokenProvider = &GeminiTokenProvider{accountRepo: repo, tokenCache: acceptanceTokenCache(e.gemini.tokenProvider.tokenCache)}
	}
	s.antigravityGatewayService = nil
	return &s
}
func (e *AccountAcceptanceExecutor) auditAntigravity(repo AccountRepository, u *acceptanceHTTPUpstream) *AntigravityGatewayService {
	s := &AntigravityGatewayService{accountRepo: repo, httpUpstream: u}
	u.base = e.antigravity.httpUpstream
	if e.antigravity.tokenProvider != nil {
		s.tokenProvider = &AntigravityTokenProvider{accountRepo: repo, tokenCache: acceptanceTokenCache(e.antigravity.tokenProvider.tokenCache)}
	}
	return s
}

// Each sample represents exactly one billed inference. In-process protocol or
// model retries are rejected at the transport boundary before network access.
type acceptanceHTTPUpstream struct {
	base      HTTPUpstream
	accountID int64
	mu        sync.Mutex
	sent      bool
	status    int
	parser    *acceptanceEvidenceParser
	result    *AcceptanceResult
	sample    AcceptanceSample
}

func (u *acceptanceHTTPUpstream) Do(r *http.Request, proxy string, id int64, n int) (*http.Response, error) {
	return u.do(r, proxy, id, n, nil)
}
func (u *acceptanceHTTPUpstream) DoWithTLS(r *http.Request, proxy string, id int64, n int, p *tlsfingerprint.Profile) (*http.Response, error) {
	return u.do(r, proxy, id, n, p)
}
func (u *acceptanceHTTPUpstream) do(r *http.Request, proxy string, id int64, n int, p *tlsfingerprint.Profile) (*http.Response, error) {
	u.mu.Lock()
	if u.sent || id != u.accountID || u.base == nil {
		u.mu.Unlock()
		return nil, errors.New("acceptance sample forbids additional upstream attempts")
	}
	u.sent = true
	u.mu.Unlock()
	r = r.WithContext(WithHTTPUpstreamRedirectsDisabled(r.Context()))
	u.result.Source = acceptanceSourceFromRequest(r, u.sample.Protocol, u.sample.Snapshot.Model, u.sample.Snapshot.UpstreamModel)
	acceptanceCaptureParameters(r, u.result)
	var resp *http.Response
	var err error
	if p == nil {
		resp, err = u.base.Do(r, proxy, id, n)
	} else {
		resp, err = u.base.DoWithTLS(r, proxy, id, n, p)
	}
	if resp != nil {
		u.status = resp.StatusCode
		if resp.Request != nil {
			u.result.Source = acceptanceSourceFromRequest(resp.Request, u.sample.Protocol, u.sample.Snapshot.Model, u.result.Source.UpstreamModel)
		}
		if resp.Body != nil && u.sample.Stream && strings.HasSuffix(r.URL.Path, "/invoke-with-response-stream") && resp.StatusCode/100 == 2 {
			resp.Body = &acceptanceBedrockBody{ReadCloser: resp.Body, parser: u.parser}
		} else if resp.Body != nil {
			resp.Body = &acceptanceObservedBody{ReadCloser: resp.Body, parser: u.parser}
		}
	}
	return resp, err
}

type acceptanceObservedBody struct {
	io.ReadCloser
	parser *acceptanceEvidenceParser
}

type acceptanceBedrockBody struct {
	io.ReadCloser
	parser *acceptanceEvidenceParser
}

func (r *acceptanceBedrockBody) observeChunk(data []byte) {
	r.parser.Feed([]byte("data: "))
	r.parser.Feed(data)
	r.parser.Feed([]byte("\n\n"))
}

func (r *acceptanceObservedBody) Read(b []byte) (int, error) {
	n, err := r.ReadCloser.Read(b)
	if n > 0 {
		r.parser.Feed(b[:n])
	}
	if err != nil {
		r.parser.Finish()
	}
	return n, err
}
func (r *acceptanceObservedBody) Close() error { r.parser.Finish(); return r.ReadCloser.Close() }

type acceptanceResponseWriter struct {
	header     http.Header
	status     int
	parser     *acceptanceEvidenceParser
	started    time.Time
	firstToken *int64
}

func (w *acceptanceResponseWriter) Header() http.Header { return w.header }
func (w *acceptanceResponseWriter) WriteHeader(n int) {
	if w.status == 0 {
		w.status = n
	}
}
func (w *acceptanceResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	w.parser.Feed(b)
	if w.firstToken == nil && (w.parser.hasText || w.parser.hasReasoning || w.parser.hasTool) {
		v := time.Since(w.started).Milliseconds()
		w.firstToken = &v
	}
	return len(b), nil
}
func (w *acceptanceResponseWriter) Flush() {}
func acceptanceCaptureParameters(r *http.Request, result *AcceptanceResult) {
	if r.GetBody == nil {
		return
	}
	body, err := r.GetBody()
	if err != nil {
		return
	}
	defer body.Close()
	d := json.NewDecoder(io.LimitReader(body, 2<<20))
	d.UseNumber()
	var m map[string]any
	if d.Decode(&m) != nil {
		return
	}
	if model := acceptanceString(m["model"]); model != "" && len(model) < 256 {
		result.Source.UpstreamModel = model
	}
	if inner := acceptanceObject(m["request"]); inner != nil {
		m = inner
	}
	input := make(map[string]any)
	for _, key := range []string{"contents", "systemInstruction", "system", "tools", "toolConfig", "messages", "tool_choice", "input", "instructions", "state", "questions"} {
		if value, ok := m[key]; ok {
			input[key] = value
		}
	}
	if len(input) > 0 {
		canonical, err := json.Marshal(input)
		if err == nil {
			hash := sha256.Sum256(canonical)
			result.EffectiveParameters["input_fingerprint"] = hex.EncodeToString(hash[:])
		}
	}
	result.EffectiveParameters["wire_model"] = result.Source.UpstreamModel
	params := make(map[string]any)
	for _, name := range []string{"reasoning", "output_config"} {
		switch effort := acceptanceString(acceptanceObject(m[name])["effort"]); effort {
		case "none", "minimal", "low", "medium", "high", "xhigh", "max", "auto":
			params[name+".effort"] = effort
		}
	}
	for _, k := range []string{"max_tokens", "max_completion_tokens", "max_output_tokens", "reasoning_effort", "stream"} {
		if v := m[k]; v != nil {
			switch v.(type) {
			case json.Number, bool:
				params[k] = v
			case string:
				if k == "reasoning_effort" {
					switch v {
					case "minimal", "low", "medium", "high", "xhigh", "max", "none", "auto":
						params[k] = v
					}
				}
			}
		}
	}
	if g := acceptanceObject(m["generationConfig"]); g != nil {
		for _, k := range []string{"maxOutputTokens", "temperature", "topP", "topK", "seed"} {
			if n, ok := g[k].(json.Number); ok {
				params[k] = n
			}
		}
		if t := acceptanceObject(g["thinkingConfig"]); t != nil {
			if v, ok := t["thinkingBudget"].(json.Number); ok {
				params["thinkingBudget"] = v
			}
			if v := acceptanceString(t["thinkingLevel"]); v == "low" || v == "medium" || v == "high" || v == "minimal" {
				params["thinkingLevel"] = v
			}
		}
	}
	result.EffectiveParameters["actual_request"] = params
}
func acceptanceBillingProjection(fr *ForwardResult, ofr *OpenAIForwardResult, raw AcceptanceUsage) AcceptanceUsage {
	p := AcceptanceUsage{Source: "billing_projection", Thoughts: raw.Thoughts, CacheRead: raw.CacheRead, CacheCreate: raw.CacheCreate, OutputIncludesThoughts: raw.OutputIncludesThoughts}
	if fr != nil {
		if raw.Input != nil || acceptanceUsageSourceKnown(fr.InputTokensSource) {
			n, _ := geminiCustomerInputTokens(fr)
			v := int64(n + fr.Usage.CacheReadInputTokens + fr.Usage.CacheCreationInputTokens)
			p.Input = &v
		}
		if raw.Output != nil || acceptanceUsageSourceKnown(fr.OutputTokensSource) {
			v := int64(fr.Usage.OutputTokens)
			if p.Thoughts != nil {
				v -= *p.Thoughts
			}
			if v >= 0 {
				p.Output = &v
			}
		}
	}
	if ofr != nil {
		if raw.Input != nil || acceptanceUsageSourceKnown(ofr.InputTokensSource) {
			v := int64(ofr.Usage.InputTokens)
			p.Input = &v
		}
		if raw.Output != nil || acceptanceUsageSourceKnown(ofr.OutputTokensSource) {
			v := int64(ofr.Usage.OutputTokens)
			if p.Thoughts != nil {
				v -= *p.Thoughts
			}
			if v >= 0 {
				p.Output = &v
			}
		}
	}
	if p.Input != nil && p.Output != nil {
		v := *p.Input + *p.Output
		if p.Thoughts != nil {
			v += *p.Thoughts
		}
		p.Total = &v
	}
	return p
}

func acceptanceCaptureBillingOrigins(result *AcceptanceResult, fr *ForwardResult, ofr *OpenAIForwardResult) {
	input, output := UsageTokenSourceUnknown, UsageTokenSourceUnknown
	if fr != nil {
		input, output = fr.InputTokensSource.Normalize(), fr.OutputTokensSource.Normalize()
	}
	if ofr != nil {
		input, output = ofr.InputTokensSource.Normalize(), ofr.OutputTokensSource.Normalize()
	}
	result.EffectiveParameters["input_tokens_source"] = string(input)
	result.EffectiveParameters["output_tokens_source"] = string(output)
	result.EffectiveParameters["billing_projection"] = map[string]any{
		"kind": "simulation", "customer_charged": false, "settlement_performed": false,
		"rule":                "production_forward_usage_and_input_discount_only",
		"input_tokens_source": string(input), "output_tokens_source": string(output),
	}
}
func acceptanceRedactExcerpt(s string, a *Account) string {
	for k, v := range a.Credentials {
		if strings.Contains(k, "key") || strings.Contains(k, "token") || strings.Contains(k, "secret") || strings.Contains(k, "password") {
			if text, ok := v.(string); ok && len(text) > 3 {
				s = strings.ReplaceAll(s, text, "[redacted]")
			}
		}
	}
	return s
}

func acceptanceRequestBody(s AcceptanceSample) ([]byte, error) {
	max := s.MaxOutputTokens
	if max <= 0 || max > 2048 {
		max = 2048
	}
	text := s.Prompt
	switch s.Protocol {
	case "systemone":
		return acceptanceSystemOneBody(s)
	case "messages":
		return acceptanceMessagesBody(s, max)
	case "responses":
		return acceptanceResponsesBody(s, max)
	case "count_tokens":
		if s.Snapshot.Platform == PlatformAnthropic {
			return acceptanceMessagesBody(s, max)
		}
	case "chat", "gemini":
	default:
		return nil, ErrAcceptanceInvalid
	}
	if s.Protocol == "chat" {
		var content any = text
		switch s.Category {
		case "image":
			content = []any{map[string]any{"type": "text", "text": text}, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64," + acceptanceImageFixture()}}}
		case "audio":
			content = []any{map[string]any{"type": "text", "text": text}, map[string]any{"type": "input_audio", "input_audio": map[string]any{"data": acceptanceAudioFixture(), "format": "wav"}}}
		}
		body := map[string]any{"model": s.Snapshot.Model, "messages": []any{map[string]any{"role": "user", "content": content}}, "stream": s.Stream, "max_tokens": max}
		if acceptanceGPTCompletionLimit(s.Snapshot.UpstreamModel) {
			delete(body, "max_tokens")
			body["max_completion_tokens"] = max
		}
		if acceptanceThinkingConfig(s.Snapshot.UpstreamModel) != nil || (s.Category != "tools" && acceptanceLowEffort(s.Snapshot.UpstreamModel)) {
			body["reasoning_effort"] = "low"
		}
		if s.Stream {
			body["stream_options"] = map[string]any{"include_usage": true}
		}
		if s.Category == "tools" {
			body["tools"] = []any{map[string]any{"type": "function", "function": map[string]any{"name": "acceptance_echo", "description": "Echo the supplied test value", "parameters": map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}}, "required": []string{"value"}}}}}
			body["tool_choice"] = map[string]any{"type": "function", "function": map[string]any{"name": "acceptance_echo"}}
		}
		return json.Marshal(body)
	}
	parts := []any{map[string]any{"text": text}}
	if s.Category == "image" {
		parts = append(parts, map[string]any{"inlineData": map[string]any{"mimeType": "image/png", "data": acceptanceImageFixture()}})
	}
	if s.Category == "audio" {
		parts = append(parts, map[string]any{"inlineData": map[string]any{"mimeType": "audio/wav", "data": acceptanceAudioFixture()}})
	}
	body := map[string]any{"contents": []any{map[string]any{"role": "user", "parts": parts}}}
	if s.Protocol != "count_tokens" {
		generation := map[string]any{"maxOutputTokens": max}
		if thinking := acceptanceThinkingConfig(s.Snapshot.UpstreamModel); thinking != nil {
			generation["thinkingConfig"] = thinking
		}
		body["generationConfig"] = generation
	}
	if s.Category == "tools" {
		body["tools"] = []any{map[string]any{"functionDeclarations": []any{map[string]any{"name": "acceptance_echo", "description": "Echo the supplied test value", "parameters": map[string]any{"type": "OBJECT", "properties": map[string]any{"value": map[string]any{"type": "STRING"}}, "required": []string{"value"}}}}}}
		body["toolConfig"] = map[string]any{"functionCallingConfig": map[string]any{"mode": "ANY", "allowedFunctionNames": []string{"acceptance_echo"}}}
	}
	return json.Marshal(body)
}
func acceptanceImageFixture() string {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return base64.StdEncoding.EncodeToString(b.Bytes())
}
func acceptanceAudioFixture() string {
	const rate = 16000
	data := make([]byte, 44+rate*2)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], rate*2)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], rate*2)
	for i := 0; i < rate; i++ {
		v := int16(4000 * math.Sin(2*math.Pi*440*float64(i)/rate))
		binary.LittleEndian.PutUint16(data[44+i*2:], uint16(v))
	}
	return base64.StdEncoding.EncodeToString(data)
}

// This decorator is private to isolated audit services. All mutating repository
// methods are explicitly overridden (including currently unused ones), so a
// forwarder's normal health bookkeeping cannot change production account state.
type acceptanceReadOnlyAccounts struct{ AccountRepository }

func (r *acceptanceReadOnlyAccounts) Create(ctx context.Context, account *Account) error { return nil }
func (r *acceptanceReadOnlyAccounts) Update(ctx context.Context, account *Account) error { return nil }
func (r *acceptanceReadOnlyAccounts) Delete(ctx context.Context, id int64) error         { return nil }
func (r *acceptanceReadOnlyAccounts) UpdateLastUsed(ctx context.Context, id int64) error { return nil }
func (r *acceptanceReadOnlyAccounts) BatchUpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) SetError(ctx context.Context, id int64, errorMsg string) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) ClearError(ctx context.Context, id int64) error { return nil }
func (r *acceptanceReadOnlyAccounts) SetSchedulable(ctx context.Context, id int64, schedulable bool) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) AutoPauseExpiredAccounts(ctx context.Context, now time.Time) (int64, error) {
	return 0, nil
}
func (r *acceptanceReadOnlyAccounts) BindGroups(ctx context.Context, accountID int64, groupIDs []int64) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) SetModelRateLimit(ctx context.Context, id int64, scope string, resetAt time.Time, reason ...string) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) SetOverloaded(ctx context.Context, id int64, until time.Time) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) ClearTempUnschedulable(ctx context.Context, id int64) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) ClearRateLimit(ctx context.Context, id int64) error { return nil }
func (r *acceptanceReadOnlyAccounts) ClearAntigravityQuotaScopes(ctx context.Context, id int64) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) ClearModelRateLimits(ctx context.Context, id int64) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) ClearModelRateLimit(ctx context.Context, id int64, scope string) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) UpdateSessionWindow(ctx context.Context, id int64, start, end *time.Time, status string) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) UpdateSessionWindowEnd(ctx context.Context, id int64, end time.Time) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) BulkUpdate(ctx context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	return 0, nil
}
func (r *acceptanceReadOnlyAccounts) IncrementQuotaUsed(ctx context.Context, id int64, amount float64) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) ResetQuotaUsedAndClearRateLimitCooldown(ctx context.Context, id int64) error {
	return nil
}
func (r *acceptanceReadOnlyAccounts) RevertProxyFallback(ctx context.Context, accountID int64) error {
	return nil
}

func acceptanceUsageSourceKnown(source UsageTokenSource) bool {
	return source == UsageTokenSourceUpstream || source == UsageTokenSourceEstimated
}

func acceptanceCacheMinimum(model string) int {
	// Official documented families, verified 2026-11-18:
	// https://aistudio.google.com/docs/caching
	// Do not infer thresholds for Lite, media variants or third-party suffixes.
	switch model {
	case "gemini-2.5-flash", "gemini-2.5-pro":
		return 2048
	case "gemini-3.1-pro-preview", "gemini-3.8-flash", "gemini-4-argon", "gemini-4-carbon":
		return 4096
	}
	return 0
}

func acceptanceThinkingConfig(model string) map[string]any {
	// Verified 2026-11-18 against the per-model ranges at:
	// https://aistudio.google.com/docs/thinking?generate-content=true
	// Unknown models receive no unsupported thinking field. Production adapters
	// still resolve their normal model mapping; we capture the effective wire config.
	switch model {
	case "gemini-2.5-flash-lite", "gemini-2.5-flash-lite-preview-06-17", "gemini-2.5-flash-lite-preview-09-2025":
		return map[string]any{"thinkingBudget": 512}
	case "gemini-2.5-flash", "gemini-2.5-pro":
		return map[string]any{"thinkingBudget": 128}
	case "gemini-3-pro-preview", "gemini-3-flash-preview", "gemini-3.1-pro-preview", "gemini-3.8-flash", "gemini-4-argon", "gemini-4-carbon":
		return map[string]any{"thinkingLevel": "low"}
	}
	return nil
}

type acceptanceReadOnlyTokenCache struct{ GeminiTokenCache }

func acceptanceTokenCache(cache GeminiTokenCache) GeminiTokenCache {
	if cache == nil {
		return nil
	}
	return acceptanceReadOnlyTokenCache{cache}
}
func (acceptanceReadOnlyTokenCache) SetAccessToken(context.Context, string, string, time.Duration) error {
	return nil
}
func (acceptanceReadOnlyTokenCache) DeleteAccessToken(context.Context, string) error { return nil }
func (acceptanceReadOnlyTokenCache) AcquireRefreshLock(context.Context, string, time.Duration) (bool, error) {
	return false, nil
}
func (acceptanceReadOnlyTokenCache) ReleaseRefreshLock(context.Context, string) error { return nil }

func acceptanceUsageContradictory(u AcceptanceUsage) bool {
	if u.Input != nil && u.CacheRead != nil && *u.CacheRead > *u.Input {
		return true
	}
	if u.Input == nil || u.Output == nil || u.Total == nil {
		return false
	}
	if *u.Input > math.MaxInt64-*u.Output {
		return true
	}
	known := *u.Input + *u.Output
	if u.Thoughts != nil {
		if known > math.MaxInt64-*u.Thoughts {
			return true
		}
		known += *u.Thoughts
	}
	// Unknown thought breakdown may explain an excess; do not fabricate a count.
	return *u.Total < known || (u.Thoughts != nil && *u.Total != known)
}
