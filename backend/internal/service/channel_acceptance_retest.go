package service

import (
	"context"
	"slices"
)

type acceptanceRetestKey struct {
	accountID                 int64
	model, category, protocol string
	stream                    bool
}

func acceptanceSampleRetestKey(s AcceptanceSample) acceptanceRetestKey {
	return acceptanceRetestKey{s.Snapshot.AccountID, s.Snapshot.Model, s.Category, s.Protocol, s.Stream}
}

func acceptanceNeedsRetest(r *AcceptanceResult) bool {
	if r == nil {
		return false
	}
	return r.Status == "anomaly" || r.Status == "error" || slices.ContainsFunc(r.Findings, func(f AcceptanceFinding) bool { return f.Level == "warning" || f.Level == "error" })
}

func (s *ChannelAcceptanceService) RetestPreview(ctx context.Context, actor int64, id string) (*AcceptancePreview, error) {
	run, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if run.StaleConfig || run.RuleVersion != ChannelAcceptanceRuleVersion {
		return nil, ErrAcceptanceStale
	}
	if (run.Status != "completed" && run.Status != "budget_exhausted" && run.Status != "cancelled") || run.Completed < run.Total {
		return nil, ErrAcceptanceInvalid
	}
	req := AcceptancePreviewRequest{BudgetUSD: run.BudgetUSD}
	selected := map[acceptanceRetestKey]bool{}
	pairedInputs := map[int64][]string{}
	for _, sample := range run.Samples {
		if !acceptanceNeedsRetest(sample.Result) {
			continue
		}
		key := acceptanceSampleRetestKey(sample)
		selected[key] = true
		// Regenerate comparable input samples and their count controls together.
		if key.category == "input" || key.category == "count" {
			pairedInputs[key.accountID] = append(pairedInputs[key.accountID], key.model)
		}
		switch sample.Category {
		case "long_context", "audio", "image", "tools":
			if !slices.Contains(req.Extras, sample.Category) {
				req.Extras = append(req.Extras, sample.Category)
			}
		}
		snap := sample.Snapshot
		if slices.ContainsFunc(req.Targets, func(t AcceptanceTargetRequest) bool { return t.AccountID == snap.AccountID && t.Model == snap.Model }) {
			continue
		}
		t := AcceptanceTargetRequest{AccountID: snap.AccountID, Model: snap.Model, DeclaredSource: snap.DeclaredSource, GroupID: snap.GroupID}
		if snap.Price.Source == "manual" {
			p := snap.Price
			t.Price = &p
		}
		req.Targets = append(req.Targets, t)
	}
	for _, sample := range run.Samples {
		if (sample.Category == "input" || sample.Category == "count") && slices.Contains(pairedInputs[sample.Snapshot.AccountID], sample.Snapshot.Model) {
			selected[acceptanceSampleRetestKey(sample)] = true
		}
	}
	return s.preview(ctx, actor, req, selected)
}
