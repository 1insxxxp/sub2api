package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

const (
	defaultCatalogReconcileInterval = 30 * time.Second
	catalogPushTimeout              = 15 * time.Second
	catalogPushMaxAttempts          = 4
	catalogPushRetryBase            = 100 * time.Millisecond
	maxSafeCatalogVersion           = int64(1 << 53)
)

// NewConfiguredCatalogPusher returns nil unless both endpoint and secret are configured.
func NewConfiguredCatalogPusher(groups GroupRepository, accounts AccountRepository) *CatalogSnapshotPusher {
	endpoint, secret := os.Getenv("MODEL_STATUS_CATALOG_ENDPOINT"), os.Getenv("MODEL_STATUS_CATALOG_SECRET")
	if endpoint == "" || secret == "" || groups == nil || accounts == nil {
		return nil
	}
	return &CatalogSnapshotPusher{Endpoint: endpoint, Secret: []byte(secret), Interval: defaultCatalogReconcileInterval, Build: buildCatalogSnapshot(groups, accounts)}
}

func buildCatalogSnapshot(groups GroupRepository, accounts AccountRepository) CatalogSnapshotBuilder {
	return func(ctx context.Context) (CatalogSnapshot, error) {
		items, err := groups.ListActive(ctx)
		if err != nil {
			return CatalogSnapshot{}, err
		}
		snapshot := CatalogSnapshot{Source: "sub2api", Groups: make([]CatalogGroup, 0, len(items))}
		for _, group := range items {
			if group.IsExclusive || group.Status != StatusActive {
				continue
			}
			bound, err := accounts.ListSchedulableByGroupID(ctx, group.ID)
			if err != nil {
				return CatalogSnapshot{}, err
			}
			models := catalogModelsForGroup(group, bound)
			if len(models) == 0 {
				continue
			}
			snapshot.Groups = append(snapshot.Groups, CatalogGroup{ID: group.ID, Name: group.Name, Platform: group.Platform, Enabled: true, IsExclusive: false, Models: models})
		}
		sort.Slice(snapshot.Groups, func(i, j int) bool { return snapshot.Groups[i].ID < snapshot.Groups[j].ID })
		return snapshot, nil
	}
}

func catalogModelsForGroup(group Group, accounts []Account) []CatalogModel {
	seen := make(map[string]struct{})
	add := func(account *Account, name string) {
		name = strings.TrimSpace(name)
		if name != "" && !shouldHideUnavailableProviderModel(account, name) {
			seen[name] = struct{}{}
		}
	}
	if group.ModelsListConfig.Enabled {
		for _, name := range group.ModelsListConfig.Models {
			for i := range accounts {
				if accountMatchesGroupPlatform(&accounts[i], group.Platform) && accounts[i].IsModelSupported(name) {
					add(&accounts[i], name)
					break
				}
			}
		}
	} else {
		for i := range accounts {
			account := &accounts[i]
			if !accountMatchesGroupPlatform(account, group.Platform) {
				continue
			}
			mapping := account.GetModelMapping()
			if len(mapping) > 0 {
				for name := range mapping {
					add(account, name)
				}
				continue
			}
			for _, name := range defaultCatalogModelIDs(account.Platform) {
				if account.IsModelSupported(name) {
					add(account, name)
				}
			}
		}
	}
	models := make([]CatalogModel, 0, len(seen))
	for name := range seen {
		models = append(models, CatalogModel{Name: name, Platform: group.Platform, Enabled: true})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
	return models
}

func accountMatchesGroupPlatform(account *Account, platform string) bool {
	if account == nil {
		return false
	}
	if platform == PlatformComposite {
		return isConcreteRequestPlatform(account.Platform)
	}
	return account.Platform == platform
}

func defaultCatalogModelIDs(platform string) []string {
	switch platform {
	case PlatformOpenAI:
		return openai.DefaultModelIDs()
	case PlatformGemini:
		ids := make([]string, 0, len(geminicli.DefaultModels))
		for _, model := range geminicli.DefaultModels {
			ids = append(ids, model.ID)
		}
		return ids
	case PlatformAntigravity:
		models := antigravity.DefaultModels()
		ids := make([]string, 0, len(models))
		for _, model := range models {
			ids = append(ids, model.ID)
		}
		return ids
	case PlatformGrok:
		return xai.DefaultModelIDs()
	case PlatformAnthropic:
		ids := make([]string, 0, len(claude.DefaultModels))
		for _, model := range claude.DefaultModels {
			ids = append(ids, model.ID)
		}
		return ids
	default:
		return nil
	}
}

type CatalogSnapshot struct {
	Source      string         `json:"source"`
	Version     int64          `json:"version"`
	GeneratedAt string         `json:"generated_at"`
	Groups      []CatalogGroup `json:"groups"`
}

type CatalogGroup struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Platform    string         `json:"platform"`
	Enabled     bool           `json:"enabled"`
	IsExclusive bool           `json:"is_exclusive"`
	Models      []CatalogModel `json:"models"`
}

type CatalogModel struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Enabled  bool   `json:"enabled"`
}
type CatalogSnapshotBuilder func(context.Context) (CatalogSnapshot, error)

// CatalogSnapshotPusher publishes complete snapshots. The dirty bit prevents
// config changes arriving during an HTTP request from being lost.
type CatalogSnapshotPusher struct {
	Endpoint string
	Secret   []byte
	Client   *http.Client
	Build    CatalogSnapshotBuilder
	Interval time.Duration
	mu       sync.Mutex
	inFlight bool
	dirty    bool
	started  bool
	stopped  bool
	stopCh   chan struct{}
	wg       sync.WaitGroup
	lastVer  int64
}

func (p *CatalogSnapshotPusher) Start() {
	if p == nil || p.Endpoint == "" || p.Build == nil {
		return
	}
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return
	}
	p.started = true
	p.stopCh = make(chan struct{})
	p.wg.Add(1)
	interval := p.Interval
	if interval <= 0 {
		interval = defaultCatalogReconcileInterval
	}
	p.mu.Unlock()
	p.PushAsync()
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.PushAsync()
			case <-p.stopCh:
				return
			}
		}
	}()
}

func (p *CatalogSnapshotPusher) Stop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.stopped = true
	if p.stopCh != nil {
		close(p.stopCh)
	}
	p.mu.Unlock()
	p.wg.Wait()
}

func (p *CatalogSnapshotPusher) PushAsync() {
	if p == nil || p.Endpoint == "" || p.Build == nil {
		return
	}
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.dirty = true
	if p.inFlight {
		p.mu.Unlock()
		return
	}
	p.inFlight = true
	p.wg.Add(1)
	p.mu.Unlock()
	go func() { defer p.wg.Done(); p.drain() }()
}

func (p *CatalogSnapshotPusher) drain() {
	defer func() { p.mu.Lock(); p.inFlight = false; p.mu.Unlock() }()
	for {
		p.mu.Lock()
		if !p.dirty || p.stopped {
			p.mu.Unlock()
			return
		}
		p.dirty = false
		p.mu.Unlock()
		if err := p.pushOnceWithRetry(); err != nil {
			p.mu.Lock()
			p.dirty = true
			p.mu.Unlock()
			return
		}
	}
}

func (p *CatalogSnapshotPusher) pushOnceWithRetry() error {
	ctx, cancel := context.WithTimeout(context.Background(), catalogPushTimeout)
	defer cancel()
	snapshot, err := p.Build(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	p.mu.Lock()
	version := now.UnixMicro()
	if version <= p.lastVer {
		version = p.lastVer + 1
	}
	p.lastVer = version
	p.mu.Unlock()
	snapshot.Source, snapshot.Version, snapshot.GeneratedAt = "sub2api", version, now.Format(time.RFC3339)
	if snapshot.Groups == nil {
		snapshot.Groups = []CatalogGroup{}
	}
	body, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	var lastErr error
	for attempt := 0; attempt < catalogPushMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Catalog-Signature", p.Signature(body))
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			err = fmt.Errorf("catalog endpoint returned HTTP %d", resp.StatusCode)
		}
		lastErr = err
		if attempt+1 < catalogPushMaxAttempts {
			timer := time.NewTimer(catalogPushRetryBase << attempt)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
		}
	}
	return lastErr
}

func (p *CatalogSnapshotPusher) Signature(body []byte) string {
	mac := hmac.New(sha256.New, p.Secret)
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func ShouldPushForSchedulerEvent(eventType string) bool {
	switch eventType {
	case SchedulerOutboxEventAccountChanged, SchedulerOutboxEventAccountGroupsChanged, SchedulerOutboxEventAccountBulkChanged, SchedulerOutboxEventGroupChanged, SchedulerOutboxEventFullRebuild:
		return true
	default:
		return false
	}
}

func ValidateCatalogSnapshot(s CatalogSnapshot) error {
	if s.Source != "sub2api" {
		return fmt.Errorf("invalid catalog source")
	}
	if s.Version <= 0 || s.Version >= maxSafeCatalogVersion {
		return fmt.Errorf("catalog version must be positive and below 2^53")
	}
	if _, err := time.Parse(time.RFC3339, s.GeneratedAt); err != nil {
		return fmt.Errorf("invalid generated_at: %w", err)
	}
	if s.Groups == nil {
		return fmt.Errorf("groups must be an array")
	}
	for _, g := range s.Groups {
		if g.ID <= 0 || g.Name == "" || g.Platform == "" || !g.Enabled || g.IsExclusive {
			return fmt.Errorf("invalid catalog group")
		}
		if g.Models == nil {
			return fmt.Errorf("catalog group models must be an array")
		}
		for _, m := range g.Models {
			if m.Name == "" || m.Platform == "" || !m.Enabled {
				return fmt.Errorf("invalid catalog model")
			}
		}
	}
	return nil
}
