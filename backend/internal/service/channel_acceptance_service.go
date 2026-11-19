package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type acceptanceAccountReader interface {
	GetByID(context.Context, int64) (*Account, error)
}
type acceptanceGroupReader interface {
	GetByID(context.Context, int64) (*Group, error)
}

type ChannelAcceptanceService struct {
	repo     ChannelAcceptanceRepository
	accounts acceptanceAccountReader
	groups   acceptanceGroupReader
	billing  *BillingService
	executor ChannelAcceptanceExecutor
	mu       sync.Mutex
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewChannelAcceptanceService(repo ChannelAcceptanceRepository, accounts acceptanceAccountReader, groups acceptanceGroupReader, billing *BillingService, executor ChannelAcceptanceExecutor) *ChannelAcceptanceService {
	return &ChannelAcceptanceService{repo: repo, accounts: accounts, groups: groups, billing: billing, executor: executor}
}

func (s *ChannelAcceptanceService) price(a *Account, model string) *AcceptancePrice {
	if s.billing == nil || !s.billing.HasIdentifiedTokenPricing(model) {
		return nil
	}
	p, err := s.billing.GetModelPricing(model)
	if err != nil || p == nil {
		return nil
	}
	rate := a.BillingRateMultiplier()
	price := &AcceptancePrice{InputPerMillion: p.InputPricePerToken * 1e6 * rate, OutputPerMillion: p.OutputPricePerToken * 1e6 * rate, CachePerMillion: p.CacheReadPricePerToken * 1e6 * rate, Source: "catalog_x_account_rate_estimate"}
	writePrice := p.CacheCreation5mPrice
	if writePrice <= 0 {
		writePrice = p.CacheCreationPricePerToken
	}
	price.CacheWritePerMillion = writePrice * 1e6 * rate
	if !acceptanceValidPrice(price) {
		return nil
	}
	return price
}

func (s *ChannelAcceptanceService) Targets(ctx context.Context, ids []int64) ([]AcceptanceTarget, error) {
	if len(ids) == 0 || len(ids) > 50 {
		return nil, ErrAcceptanceInvalid
	}
	items := make([]AcceptanceTarget, 0, len(ids))
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		a, err := s.accounts.GetByID(ctx, id)
		if err != nil || a == nil {
			return nil, ErrAcceptanceNotFound
		}
		target := AcceptanceTarget{AccountID: a.ID, Name: a.Name, Platform: a.Platform, Models: []AcceptanceModel{}, Groups: []AcceptanceGroup{}}
		groupIDs := append([]int64(nil), a.GroupIDs...)
		for _, ag := range a.AccountGroups {
			groupIDs = append(groupIDs, ag.GroupID)
		}
		groupSeen := map[int64]bool{}
		var groupModels []string
		for _, id := range groupIDs {
			if groupSeen[id] || s.groups == nil {
				continue
			}
			groupSeen[id] = true
			if group, err := s.groups.GetByID(ctx, id); err == nil && group != nil {
				target.Groups = append(target.Groups, AcceptanceGroup{ID: id, Name: group.Name})
				if allowlist := group.EffectiveModelAllowlist(); allowlist.Enabled {
					groupModels = append(groupModels, allowlist.Models...)
				}
			}
		}
		target.UnavailableReason = acceptanceAccountReason(a)
		if target.UnavailableReason == "" {
			for _, id := range acceptanceModelIDs(a, groupModels) {
				model := acceptanceModelPlan(a, id, nil)
				if model.UnavailableReason == "" {
					model.Price = s.price(a, model.UpstreamModel)
				}
				target.Models = append(target.Models, model)
			}
			if len(target.Models) == 0 {
				target.UnavailableReason = "未找到能匹配账号映射的具体模型名；请配置具体模型或在分组模型清单中指定通配符匹配项"
			}
		}
		items = append(items, target)
	}
	return items, nil
}

// Hash only evidence-relevant configuration. Runtime counters and short-lived OAuth
// access tokens must not invalidate a preview while an existing account serves users.
func AcceptanceAccountFingerprint(a *Account, g *Group) string {
	if a == nil {
		return ""
	}
	if g != nil {
		copyGroup := *g
		copyGroup.AccountCount, copyGroup.ActiveAccountCount, copyGroup.RateLimitedAccountCount = 0, 0, 0
		copyGroup.AvailableModelCount = 0
		copyGroup.AccountGroups = nil
		copyGroup.Hydrated = false
		copyGroup.CreatedAt, copyGroup.UpdatedAt = time.Time{}, time.Time{}
		g = &copyGroup
	}
	credentials := map[string]any{}
	for k, v := range a.Credentials {
		switch k {
		case "access_token", "expires_at", "expires_in", "token_expiry", "last_refresh_at", "last_refresh_time":
			continue
		}
		credentials[k] = v
	}
	extra := map[string]any{}
	for k, v := range a.Extra {
		if strings.HasPrefix(k, "last_") {
			continue
		}
		switch k {
		case "upstream_billing_probe", "oauth_usage", "usage_snapshot", "quota_snapshot", "codex_usage", "antigravity_usage":
			continue
		}
		extra[k] = v
	}
	value := struct {
		ID                 int64
		Platform, Type     string
		Credentials, Extra map[string]any
		Prompts            map[string]string
		Proxy              *Proxy
		ProxyID            *int64
		Rate               float64
		Group              *Group
	}{a.ID, a.Platform, a.Type, credentials, extra, a.ModelSystemPrompts, a.Proxy, a.ProxyID, a.BillingRateMultiplier(), g}
	b, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (s *ChannelAcceptanceService) targetSnapshot(ctx context.Context, t AcceptanceTargetRequest) (AcceptanceSnapshot, error) {
	a, err := s.accounts.GetByID(ctx, t.AccountID)
	if err != nil || a == nil {
		return AcceptanceSnapshot{}, ErrAcceptanceNotFound
	}
	if acceptanceAccountReason(a) != "" || !acceptanceConcreteID(t.Model) || !isModelSupportedByAccount(a, t.Model) {
		return AcceptanceSnapshot{}, ErrAcceptanceInvalid
	}
	var g *Group
	if t.GroupID != nil {
		bound := false
		for _, id := range a.GroupIDs {
			if id == *t.GroupID {
				bound = true
			}
		}
		for _, ag := range a.AccountGroups {
			if ag.GroupID == *t.GroupID {
				bound = true
			}
		}
		if !bound || s.groups == nil {
			return AcceptanceSnapshot{}, ErrAcceptanceInvalid
		}
		g, err = s.groups.GetByID(ctx, *t.GroupID)
		if err != nil || g == nil {
			return AcceptanceSnapshot{}, ErrAcceptanceInvalid
		}
	}
	plan := acceptanceModelPlan(a, t.Model, g)
	if plan.UnavailableReason != "" || len(plan.Protocols) == 0 {
		return AcceptanceSnapshot{}, ErrAcceptanceInvalid
	}
	mapped := plan.UpstreamModel
	price := s.price(a, mapped)
	if t.Price != nil && t.Price.Source == "manual" {
		if !acceptanceValidPrice(t.Price) {
			return AcceptanceSnapshot{}, ErrAcceptanceInvalid
		}
		v := *t.Price
		price = &v
	}
	snapshot := AcceptanceSnapshot{AccountID: a.ID, AccountName: a.Name, Platform: a.Platform, AccountType: a.Type, Protocols: plan.Protocols, Model: t.Model, UpstreamModel: mapped, GroupID: t.GroupID, Fingerprint: AcceptanceAccountFingerprint(a, g)}
	if injected, ok := a.ResolveModelSystemPrompt(mapped); ok {
		snapshot.AdditionalInputEstimate = len([]byte(injected)) + 64
	}
	if snapshot.Fingerprint == "" {
		return AcceptanceSnapshot{}, ErrAcceptanceInvalid
	}
	if price != nil {
		snapshot.Price = *price
	}
	return snapshot, nil
}

func (s *ChannelAcceptanceService) Preview(ctx context.Context, actor int64, req AcceptancePreviewRequest) (*AcceptancePreview, error) {
	if actor <= 0 || len(req.Targets) == 0 || len(req.Targets) > 25 || len(req.Extras) > 4 {
		return nil, ErrAcceptanceInvalid
	}
	if req.BudgetUSD != nil && (*req.BudgetUSD <= 0 || math.IsNaN(*req.BudgetUSD) || math.IsInf(*req.BudgetUSD, 0) || *req.BudgetUSD > 1e6) {
		return nil, ErrAcceptanceInvalid
	}
	extraSet := map[string]bool{}
	for _, e := range req.Extras {
		if extraSet[e] {
			return nil, ErrAcceptanceInvalid
		}
		extraSet[e] = true
		switch e {
		case "long_context", "image", "audio", "tools":
		default:
			return nil, ErrAcceptanceInvalid
		}
	}
	now := time.Now().UTC()
	p := &AcceptancePreview{ID: uuid.NewString(), ActorID: actor, RuleVersion: ChannelAcceptanceRuleVersion, CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute), Request: req, CanStart: true, Issues: []string{}, Samples: []AcceptanceSample{}}
	seen := map[string]bool{}
	for _, target := range req.Targets {
		key := fmt.Sprintf("%d:%s", target.AccountID, target.Model)
		if seen[key] {
			return nil, ErrAcceptanceInvalid
		}
		seen[key] = true
		snapshot, err := s.targetSnapshot(ctx, target)
		if err != nil {
			return nil, err
		}
		if !acceptanceValidPrice(&snapshot.Price) {
			p.CanStart = false
			p.Issues = append(p.Issues, fmt.Sprintf("账号 %d 模型 %s 缺少上游成本价格", target.AccountID, target.Model))
		}
		if snapshot.Platform == PlatformTypeSafe {
			for _, extra := range req.Extras {
				if extra != "long_context" {
					p.Issues = append(p.Issues, fmt.Sprintf("账号 %d 的 System One 接口不支持 %s 项目，已跳过", snapshot.AccountID, extra))
				}
			}
		}
		previousGenerationCalls := p.GenerationCalls
		for _, sample := range acceptanceManifest(snapshot, req.Extras, p.ID) {
			sample.Ordinal = len(p.Samples)
			p.Samples = append(p.Samples, sample)
			p.EstimatedCostUSD += sample.EstimatedCostUSD
			if sample.Protocol == "count_tokens" {
				p.CountCalls++
			} else {
				p.GenerationCalls++
			}
		}
		if p.GenerationCalls == previousGenerationCalls {
			return nil, ErrAcceptanceInvalid
		}
	}
	if req.BudgetUSD != nil && *req.BudgetUSD < p.EstimatedCostUSD {
		p.Issues = append(p.Issues, "预算不足以覆盖全部项目，达到预算后将停止派发剩余测试")
	}
	if p.CountCalls > 0 {
		p.Issues = append(p.Issues, "计数请求也预留误转生成的费用；计数接口返回生成内容或不可用时，停止该来源后续计数测试")
	} else {
		p.Issues = append(p.Issues, "所选来源没有可用的原生计数验收接口，本次仅发送生成请求")
	}
	if err := s.repo.SavePreview(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *ChannelAcceptanceService) Start(ctx context.Context, actor int64, previewID string) (*AcceptanceRun, error) {
	p, err := s.repo.GetPreview(ctx, previewID, actor)
	if err != nil {
		return nil, err
	}
	if existing, err := s.repo.FindRunByPreview(ctx, previewID, actor); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}
	if p.RuleVersion != ChannelAcceptanceRuleVersion || !time.Now().Before(p.ExpiresAt) {
		return nil, ErrAcceptanceStale
	}
	if !p.CanStart {
		return nil, ErrAcceptancePriceRequired
	}
	seen := map[string]AcceptanceSnapshot{}
	for _, sample := range p.Samples {
		seen[fmt.Sprintf("%d:%s", sample.Snapshot.AccountID, sample.Snapshot.Model)] = sample.Snapshot
	}
	for _, target := range p.Request.Targets {
		current, e := s.targetSnapshot(ctx, target)
		if e != nil {
			return nil, ErrAcceptanceStale
		}
		old := seen[fmt.Sprintf("%d:%s", target.AccountID, target.Model)]
		if current.Fingerprint != old.Fingerprint || current.Price != old.Price || current.UpstreamModel != old.UpstreamModel || !slices.Equal(current.Protocols, old.Protocols) {
			return nil, ErrAcceptanceStale
		}
	}
	return s.repo.CreateRun(ctx, p)
}

func (s *ChannelAcceptanceService) Get(ctx context.Context, id string) (*AcceptanceRun, error) {
	r, err := s.repo.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	analyzeAcceptanceRun(r)
	seen := map[string]bool{}
	for _, sample := range r.Samples {
		snap := sample.Snapshot
		key := fmt.Sprintf("%d:%s", snap.AccountID, snap.Model)
		if seen[key] {
			continue
		}
		seen[key] = true
		t := AcceptanceTargetRequest{AccountID: snap.AccountID, Model: snap.Model, GroupID: snap.GroupID}
		if snap.Price.Source == "manual" {
			p := snap.Price
			t.Price = &p
		}
		current, e := s.targetSnapshot(ctx, t)
		if e != nil || current.Fingerprint != snap.Fingerprint || current.Price != snap.Price {
			r.StaleConfig = true
		}
	}
	return r, nil
}
func (s *ChannelAcceptanceService) History(ctx context.Context, ids []int64) ([]AcceptanceRun, error) {
	return s.repo.ListRuns(ctx, ids, 50)
}
func (s *ChannelAcceptanceService) Cancel(ctx context.Context, id string) (*AcceptanceRun, error) {
	if err := s.repo.CancelRun(ctx, id); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *ChannelAcceptanceService) StartWorkers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil || s.executor == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	for i := 0; i < 2; i++ {
		s.wg.Add(1)
		go s.worker(ctx, uuid.NewString())
	}
}
func (s *ChannelAcceptanceService) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		s.wg.Wait()
	}
}
func (s *ChannelAcceptanceService) worker(ctx context.Context, worker string) {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastCleanup := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().UTC()
			if err := s.repo.RecoverExpired(ctx, now); err != nil {
				continue
			}
			if now.Sub(lastCleanup) > time.Hour {
				_ = s.repo.Cleanup(ctx, now.Add(-30*24*time.Hour))
				lastCleanup = now
			}
			sample, err := s.repo.ClaimNext(ctx, worker, now)
			if err != nil || sample == nil {
				continue
			}
			s.execute(ctx, *sample)
		}
	}
}
func (s *ChannelAcceptanceService) execute(ctx context.Context, sample AcceptanceSample) {
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(sample.TimeoutSeconds)*time.Second)
	defer cancel()
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-finished:
				return
			case <-requestCtx.Done():
				return
			case <-t.C:
				r, err := s.repo.GetRun(requestCtx, sample.RunID)
				if err == nil && (r.Status == "cancelled" || r.Status == "cancelling" || r.Status == "cancel_requested") {
					cancel()
					return
				}
			}
		}
	}()
	snapshot, err := s.targetSnapshot(requestCtx, AcceptanceTargetRequest{AccountID: sample.Snapshot.AccountID, Model: sample.Snapshot.Model, GroupID: sample.Snapshot.GroupID, Price: &sample.Snapshot.Price})
	var result *AcceptanceResult
	if err != nil || snapshot.Fingerprint != sample.Snapshot.Fingerprint || snapshot.Price != sample.Snapshot.Price || snapshot.UpstreamModel != sample.Snapshot.UpstreamModel || !slices.Equal(snapshot.Protocols, sample.Snapshot.Protocols) {
		zero := float64(0)
		result = &AcceptanceResult{Status: "inconclusive", CostUSD: &zero, Findings: []AcceptanceFinding{{Code: "configuration_changed", Level: "warning", Message: "配置或价格已变化，本样本未发送；请重新预览"}}}
	} else {
		if sample.Protocol == "count_tokens" {
			previous, historyErr := s.repo.GetRun(requestCtx, sample.RunID)
			if historyErr != nil || acceptanceCountSourceRejected(previous, sample) {
				zero := float64(0)
				result = &AcceptanceResult{Status: "unsupported", CostUSD: &zero, Findings: []AcceptanceFinding{{Code: "count_not_dispatched", Level: "info", Message: "先前计数结果不可靠或无法确认，本样本未发送；不使用生成接口代替计数"}}}
			}
		}
		if result == nil {
			result, err = s.executor.Execute(requestCtx, sample)
		}
	}
	if result == nil {
		status := "error"
		if errors.Is(requestCtx.Err(), context.Canceled) {
			status = "cancelled"
		}
		result = &AcceptanceResult{Status: status, Findings: []AcceptanceFinding{{Code: "execution_failed", Level: "warning", Message: "检测未完成，无法确认上游消耗；保留费用预留且不自动重放"}}}
	}
	// Interim cumulative usage can be internally consistent but still precede a
	// disconnect. Only terminal upstream usage may release the reserved budget.
	if result.CostUSD == nil && result.UpstreamUsageFinal {
		result.CostUSD = AcceptanceCalculatedCost(sample.Snapshot.Price, result.RawUsage)
	}
	completeCtx, completeCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer completeCancel()
	_ = s.repo.CompleteSample(completeCtx, sample.ID, sample.LeaseToken, result)
}

func acceptanceCountSourceRejected(run *AcceptanceRun, sample AcceptanceSample) bool {
	if run == nil {
		return true
	}
	for _, previous := range run.Samples {
		if previous.Ordinal >= sample.Ordinal || previous.Protocol != "count_tokens" || previous.Snapshot.AccountID != sample.Snapshot.AccountID || previous.Snapshot.Model != sample.Snapshot.Model || previous.Result == nil {
			continue
		}
		if previous.Result.Status != "passed" || previous.Result.RawUsage.Source != "upstream_count_tokens" {
			return true
		}
	}
	return false
}

// Raw input includes cached tokens; output excludes thought tokens in this normalized view.
func AcceptanceCalculatedCost(p AcceptancePrice, u AcceptanceUsage) *float64 {
	if !acceptanceValidPrice(&p) || u.Input == nil || u.Output == nil {
		return nil
	}
	input, output := *u.Input, *u.Output
	if input < 0 || output < 0 {
		return nil
	}
	cache := int64(0)
	if u.CacheRead != nil {
		cache = *u.CacheRead
		if cache < 0 || cache > input {
			return nil
		}
	}
	written := int64(0)
	if u.CacheCreate != nil {
		written = *u.CacheCreate
		if written < 0 || written > input-cache || (written > 0 && p.CacheWritePerMillion == 0 && p.InputPerMillion > 0) {
			return nil
		}
	}
	thoughts := int64(0)
	if u.Thoughts != nil {
		thoughts = *u.Thoughts
		if thoughts < 0 {
			return nil
		}
	} else if !u.OutputIncludesThoughts && u.Source != "upstream_chat_reported" {
		if u.Total == nil || *u.Total != input+output {
			return nil
		}
	}
	if u.Total != nil && *u.Total != input+output+thoughts {
		return nil
	}
	cost := (float64(input-cache-written)*p.InputPerMillion + float64(written)*p.CacheWritePerMillion + float64(cache)*p.CachePerMillion + (float64(output)+float64(thoughts))*p.OutputPerMillion) / 1e6
	if math.IsNaN(cost) || math.IsInf(cost, 0) {
		return nil
	}
	return &cost
}
