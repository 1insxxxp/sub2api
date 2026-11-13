package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// A source clock is bumped in the same transaction as every declaration write.
// Locking it also makes a SERIALIZABLE publication reject insertion/restore
// races that committed after its initial configuration snapshot was acquired.
func categoryDefaultLockSourceClocks(ctx context.Context, client *dbent.Client, ids []int64) error {
	rows, err := client.QueryContext(ctx, `SELECT source_group_id FROM category_default_source_clocks WHERE source_group_id=ANY($1) ORDER BY source_group_id FOR SHARE`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count != len(ids) {
		return service.ErrCategoryDefaultStalePreview
	}
	return nil
}

func categoryDefaultSourceEpoch(ctx context.Context, client *dbent.Client, id int64) (int64, error) {
	rows, err := client.QueryContext(ctx, `SELECT epoch FROM category_default_source_clocks WHERE source_group_id=$1`, id)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, service.ErrCategoryDefaultStalePreview
	}
	var epoch int64
	if err := rows.Scan(&epoch); err != nil {
		return 0, err
	}
	return epoch, rows.Err()
}

type categoryDefaultHistoricalSource struct {
	Group              *dbent.Group                  `json:"group"`
	Accounts           []service.Account             `json:"accounts"`
	Routes             []service.CompositeModelRoute `json:"routes"`
	Channel            *service.Channel              `json:"channel"`
	GrokMappingOptions *xai.ModelMappingOptions      `json:"grok_mapping_options"`
}

func categoryDefaultDecodeHistoricalSource(raw []byte) (service.CategoryDefaultSourceSnapshot, error) {
	var historical categoryDefaultHistoricalSource
	if err := json.Unmarshal(raw, &historical); err != nil {
		return service.CategoryDefaultSourceSnapshot{}, fmt.Errorf("decode category default declaration history: %w", err)
	}
	snapshot := service.CategoryDefaultSourceSnapshot{Routes: historical.Routes, Channel: historical.Channel, GrokMappingOptions: historical.GrokMappingOptions}
	if historical.Group == nil || historical.Group.DeletedAt != nil {
		return snapshot, nil
	}
	snapshot.Group = groupEntityToService(historical.Group)
	for _, account := range historical.Accounts {
		matches := (snapshot.Group.Platform == service.PlatformComposite && domain.IsConcretePlatform(account.Platform)) || account.Platform == snapshot.Group.Platform
		if account.Platform == service.PlatformAntigravity && (snapshot.Group.Platform == service.PlatformGemini || snapshot.Group.Platform == service.PlatformAnthropic) {
			matches = true
		}
		if matches {
			snapshot.Accounts = append(snapshot.Accounts, account)
		}
	}
	return snapshot, nil
}

func categoryDefaultHistorySourceEligible(category string, group *service.Group) bool {
	return group != nil && group.ID > 0 && group.Status == service.StatusActive &&
		group.SubscriptionType == service.SubscriptionTypeStandard && !group.IsCategoryDefault &&
		!group.SystemCustomRoutingEnabled && service.EffectiveKeyDisplayCategory(group) == category &&
		(domain.IsConcretePlatform(group.Platform) || group.Platform == service.PlatformComposite)
}

func categoryDefaultHistoryInvalidations(config *service.CategoryDefaultGroupConfig, sourceID int64, snapshot service.CategoryDefaultSourceSnapshot) map[string]bool {
	invalid := make(map[string]bool)
	declarations := make(map[string]service.CategoryDefaultPublishedModel)
	if categoryDefaultHistorySourceEligible(config.Category, snapshot.Group) {
		models, err := service.CategoryDefaultSourceDeclarations(snapshot)
		if err == nil {
			for _, model := range models {
				declarations[strings.ToLower(strings.TrimSpace(model.PublicModel))] = model
			}
		}
	}
	for _, published := range config.Models {
		key := strings.ToLower(strings.TrimSpace(published.PublicModel))
		current, exists := declarations[key]
		if published.SourceGroupID == sourceID {
			if !exists || current.SourceGroupID != published.SourceGroupID || current.SourceModel != published.SourceModel || current.Fingerprint != published.Fingerprint {
				invalid[key] = true
			}
		} else if exists {
			// Even an unpublished declaration in another associated source makes
			// this one public name ambiguous. A later removal cannot auto-restore it.
			invalid[key] = true
		}
	}
	return invalid
}

func categoryDefaultLoadInvalidations(ctx context.Context, client *dbent.Client, config *service.CategoryDefaultGroupConfig) error {
	config.InvalidatedModels = []string{}
	rows, err := client.QueryContext(ctx, `SELECT h.source_group_id,h.snapshot FROM category_default_declaration_history h
		JOIN category_default_groups c ON c.group_id=h.entry_group_id AND c.publication_revision=h.publication_revision
		WHERE h.entry_group_id=$1`, config.GroupID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	invalid := make(map[string]bool)
	for rows.Next() {
		var sourceID int64
		var raw []byte
		if err := rows.Scan(&sourceID, &raw); err != nil {
			return err
		}
		snapshot, err := categoryDefaultDecodeHistoricalSource(raw)
		if err != nil {
			return err
		}
		for model := range categoryDefaultHistoryInvalidations(config, sourceID, snapshot) {
			invalid[model] = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for model := range invalid {
		config.InvalidatedModels = append(config.InvalidatedModels, model)
	}
	sort.Strings(config.InvalidatedModels)
	return nil
}

// Source removers lock all associated entries before any declaration writes.
// The reference advisory lock must already be held, matching Publish's order.
func categoryDefaultLockEntriesForSourceRemoval(ctx context.Context, client *dbent.Client, sourceID int64) error {
	rows, err := client.QueryContext(ctx, `SELECT c.group_id FROM category_default_groups c
 WHERE EXISTS (SELECT 1 FROM category_default_group_sources s WHERE s.group_id=c.group_id AND s.source_group_id=$1)
 ORDER BY c.group_id FOR UPDATE OF c`, sourceID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
	}
	return rows.Err()
}
