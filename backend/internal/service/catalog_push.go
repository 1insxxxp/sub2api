package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"
)

// NewConfiguredCatalogPusher returns nil unless both endpoint and secret are
// configured. The secret is read only from the process environment.
func NewConfiguredCatalogPusher(groups GroupRepository, accounts AccountRepository) *CatalogSnapshotPusher {
	endpoint, secret := os.Getenv("MODEL_STATUS_CATALOG_ENDPOINT"), os.Getenv("MODEL_STATUS_CATALOG_SECRET")
	if endpoint == "" || secret == "" || groups == nil || accounts == nil {
		return nil
	}
	return &CatalogSnapshotPusher{Endpoint: endpoint, Secret: []byte(secret), Build: func(ctx context.Context) (CatalogSnapshot, error) {
		items, err := groups.ListActive(ctx)
		if err != nil {
			return CatalogSnapshot{}, err
		}
		s := CatalogSnapshot{Source: "sub2api", Version: time.Now().UnixNano()}
		for _, g := range items {
			if g.IsExclusive || g.Status != StatusActive || !g.ModelsListConfig.Enabled {
				continue
			}
			bound, err := accounts.ListSchedulableByGroupID(ctx, g.ID)
			if err != nil {
				return CatalogSnapshot{}, err
			}
			if len(bound) == 0 {
				continue
			}
			seen := map[string]bool{}
			models := make([]CatalogModel, 0, len(g.ModelsListConfig.Models))
			for _, name := range g.ModelsListConfig.Models {
				if name != "" && !seen[name] {
					seen[name] = true
					models = append(models, CatalogModel{Name: name})
				}
			}
			if len(models) == 0 {
				continue
			}
			sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
			s.Groups = append(s.Groups, CatalogGroup{ID: g.ID, Name: g.Name, Models: models})
		}
		sort.Slice(s.Groups, func(i, j int) bool { return s.Groups[i].ID < s.Groups[j].ID })
		return s, nil
	}}
}

// CatalogSnapshot is the public, filtered model directory consumed by the
// model-status-report project. Keep this independent from database entities so
// credentials and internal routing details can never leak over the wire.
type CatalogSnapshot struct {
	Source  string         `json:"source"`
	Version int64          `json:"version"`
	Groups  []CatalogGroup `json:"groups"`
}

type CatalogGroup struct {
	ID     int64          `json:"id"`
	Name   string         `json:"name"`
	Models []CatalogModel `json:"models"`
}

type CatalogModel struct {
	Name string `json:"name"`
}

// CatalogSnapshotBuilder must return a complete current snapshot. It is
// invoked after any scheduler outbox configuration event, not in the admin
// request path.
type CatalogSnapshotBuilder func(context.Context) (CatalogSnapshot, error)

// CatalogSnapshotPusher asynchronously publishes complete snapshots. A full
// replacement makes rename/disable operations immediately visible and avoids
// ordering bugs from patch events.
type CatalogSnapshotPusher struct {
	Endpoint string
	Secret   []byte
	Client   *http.Client
	Build    CatalogSnapshotBuilder
	Queue    chan struct{}
}

func (p *CatalogSnapshotPusher) PushAsync() {
	if p == nil || p.Endpoint == "" || p.Build == nil {
		return
	}
	if p.Queue == nil {
		p.Queue = make(chan struct{}, 1)
	}
	select {
	case p.Queue <- struct{}{}:
	default:
		return
	}
	go func() {
		defer func() { <-p.Queue }()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		snapshot, err := p.Build(ctx)
		if err != nil {
			return
		}
		body, err := json.Marshal(snapshot)
		if err != nil {
			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Catalog-Signature", p.Signature(body))
		client := p.Client
		if client == nil {
			client = http.DefaultClient
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()
}

func (p *CatalogSnapshotPusher) Signature(body []byte) string {
	mac := hmac.New(sha256.New, p.Secret)
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// ShouldPushForSchedulerEvent identifies mutations which can change the
// visible model catalog. Last-used events are intentionally ignored.
func ShouldPushForSchedulerEvent(eventType string) bool {
	switch eventType {
	case SchedulerOutboxEventAccountChanged, SchedulerOutboxEventAccountGroupsChanged,
		SchedulerOutboxEventAccountBulkChanged, SchedulerOutboxEventGroupChanged,
		SchedulerOutboxEventFullRebuild:
		return true
	default:
		return false
	}
}

func ValidateCatalogSnapshot(s CatalogSnapshot) error {
	if s.Source != "sub2api" {
		return fmt.Errorf("invalid catalog source")
	}
	if s.Version <= 0 {
		return fmt.Errorf("catalog version must be positive")
	}
	for _, g := range s.Groups {
		if g.ID <= 0 || g.Name == "" {
			return fmt.Errorf("invalid catalog group")
		}
		for _, m := range g.Models {
			if m.Name == "" {
				return fmt.Errorf("invalid catalog model")
			}
		}
	}
	return nil
}
