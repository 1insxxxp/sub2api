package service

import (
	"encoding/json"
	"testing"
	"time"
)

func TestOpsUpstreamErrorSummaryContract(t *testing.T) {
	latest := time.Date(2026, 9, 25, 12, 34, 56, 0, time.UTC)
	groupID := int64(7)
	accountID := int64(42)
	summary := OpsUpstreamErrorSummary{
		TotalErrors: 9, GroupCount: 1, LatestAt: &latest,
		Groups: []*OpsUpstreamErrorSummaryGroup{{
			GroupID: &groupID, GroupName: "production", ErrorCount: 9, ModelCount: 1, AccountCount: 2, LatestAt: &latest,
			Models: []*OpsUpstreamErrorSummaryModel{{
				Model: "gpt-5", ErrorCount: 9, LatestAt: &latest, StatusCodes: map[int]int64{429: 6, 500: 3},
				Accounts: []*OpsUpstreamErrorSummaryAccount{{
					AccountID: &accountID, AccountName: "primary", ErrorCount: 9, LatestAt: &latest, LatestStatusCode: 500,
					Reasons: []*OpsUpstreamErrorSummaryReason{{Message: "upstream overloaded", ErrorType: "upstream_error", StatusCode: 500, Count: 3, LatestAt: &latest, RepresentativeErrorID: 99}},
				}},
			}},
		}},
	}

	payload, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	for _, key := range []string{"total_errors", "group_count", "latest_at", "groups"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("summary JSON missing %q: %s", key, payload)
		}
	}
	groups, ok := got["groups"].([]any)
	if !ok || len(groups) != 1 {
		t.Fatalf("groups = %#v, want one group", got["groups"])
	}
	group := groups[0].(map[string]any)
	if group["error_count"] != float64(9) || group["model_count"] != float64(1) || group["account_count"] != float64(2) {
		t.Fatalf("group counts = %#v", group)
	}
	model := group["models"].([]any)[0].(map[string]any)
	if model["status_codes"].(map[string]any)["429"] != float64(6) {
		t.Fatalf("status code counts = %#v", model["status_codes"])
	}
	account := model["accounts"].([]any)[0].(map[string]any)
	reason := account["reasons"].([]any)[0].(map[string]any)
	if reason["representative_error_id"] != float64(99) || reason["message"] != "upstream overloaded" {
		t.Fatalf("reason = %#v", reason)
	}
}

func TestOpsUpstreamErrorSummaryEmptyResultUsesEmptyArrays(t *testing.T) {
	empty := OpsUpstreamErrorSummary{Groups: []*OpsUpstreamErrorSummaryGroup{}}
	payload, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal empty summary: %v", err)
	}
	var got struct {
		Groups []*OpsUpstreamErrorSummaryGroup `json:"groups"`
	}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal empty summary: %v", err)
	}
	if got.Groups == nil || len(got.Groups) != 0 {
		t.Fatalf("groups = %#v, want non-nil empty array", got.Groups)
	}
}
