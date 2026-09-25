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
	group, ok := groups[0].(map[string]any)
	if !ok {
		t.Fatalf("group = %#v", groups[0])
	}
	if group["error_count"] != float64(9) || group["model_count"] != float64(1) || group["account_count"] != float64(2) {
		t.Fatalf("group counts = %#v", group)
	}
	models, ok := group["models"].([]any)
	if !ok || len(models) == 0 {
		t.Fatalf("models = %#v", group["models"])
	}
	model, ok := models[0].(map[string]any)
	if !ok {
		t.Fatalf("model = %#v", models[0])
	}
	statusCodes, ok := model["status_codes"].(map[string]any)
	if !ok || statusCodes["429"] != float64(6) {
		t.Fatalf("status code counts = %#v", model["status_codes"])
	}
	accounts, ok := model["accounts"].([]any)
	if !ok || len(accounts) == 0 {
		t.Fatalf("accounts = %#v", model["accounts"])
	}
	account, ok := accounts[0].(map[string]any)
	if !ok {
		t.Fatalf("account = %#v", accounts[0])
	}
	reasons, ok := account["reasons"].([]any)
	if !ok || len(reasons) == 0 {
		t.Fatalf("reasons = %#v", account["reasons"])
	}
	reason, ok := reasons[0].(map[string]any)
	if !ok {
		t.Fatalf("reason = %#v", reasons[0])
	}
	if reason["representative_error_id"] != float64(99) || reason["message"] != "upstream overloaded" {
		t.Fatalf("reason = %#v", reason)
	}
}

func TestOpsUpstreamErrorSummaryEmptyResultUsesEmptyArrays(t *testing.T) {
	empty := OpsUpstreamErrorSummary{}
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

func TestOpsUpstreamErrorSummaryContractMultipleGroupsModelsAccountsAndReasons(t *testing.T) {
	latest := time.Date(2026, 9, 25, 13, 0, 0, 0, time.UTC)
	group1, group2 := int64(7), int64(8)
	account1, account2, account3 := int64(41), int64(42), int64(43)
	summary := OpsUpstreamErrorSummary{
		TotalErrors: 14,
		GroupCount:  2,
		LatestAt:    &latest,
		Groups: []*OpsUpstreamErrorSummaryGroup{
			{
				GroupID: &group1, GroupName: "production", ErrorCount: 10, ModelCount: 2, AccountCount: 2, LatestAt: &latest,
				Models: []*OpsUpstreamErrorSummaryModel{
					{Model: "gpt-5", ErrorCount: 8, LatestAt: &latest, StatusCodes: map[int]int64{429: 5, 500: 3}, Accounts: []*OpsUpstreamErrorSummaryAccount{
						{AccountID: &account1, AccountName: "primary", ErrorCount: 5, LatestAt: &latest, LatestStatusCode: 429, Reasons: []*OpsUpstreamErrorSummaryReason{
							{Message: "rate limited", ErrorType: "upstream_error", StatusCode: 429, Count: 5, LatestAt: &latest, RepresentativeErrorID: 101},
						}},
						{AccountID: &account2, AccountName: "backup", ErrorCount: 3, LatestAt: &latest, LatestStatusCode: 500, Reasons: []*OpsUpstreamErrorSummaryReason{
							{Message: "provider failed", ErrorType: "upstream_error", StatusCode: 500, Count: 2, LatestAt: &latest, RepresentativeErrorID: 102},
							{Message: "connection reset", ErrorType: "network_error", StatusCode: 502, Count: 1, LatestAt: &latest, RepresentativeErrorID: 103},
						}},
					}},
					{Model: "claude-4", ErrorCount: 2, LatestAt: &latest, StatusCodes: map[int]int64{503: 2}, Accounts: []*OpsUpstreamErrorSummaryAccount{}},
				},
			},
			{
				GroupID: &group2, GroupName: "staging", ErrorCount: 4, ModelCount: 1, AccountCount: 1, LatestAt: &latest,
				Models: []*OpsUpstreamErrorSummaryModel{{Model: "gpt-5", ErrorCount: 4, LatestAt: &latest, StatusCodes: map[int]int64{429: 4}, Accounts: []*OpsUpstreamErrorSummaryAccount{{AccountID: &account3, AccountName: "staging", ErrorCount: 4, LatestAt: &latest, LatestStatusCode: 429, Reasons: []*OpsUpstreamErrorSummaryReason{{Message: "rate limited", ErrorType: "upstream_error", StatusCode: 429, Count: 4, LatestAt: &latest, RepresentativeErrorID: 104}}}}}},
			},
		},
	}

	payload, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	if got["total_errors"] != float64(14) || got["group_count"] != float64(2) {
		t.Fatalf("summary totals = %#v", got)
	}
	groups, ok := got["groups"].([]any)
	if !ok || len(groups) != 2 {
		t.Fatalf("groups = %#v", got["groups"])
	}
	groupJSON0, ok := groups[0].(map[string]any)
	if !ok {
		t.Fatalf("group[0] = %#v", groups[0])
	}
	groupJSON1, ok := groups[1].(map[string]any)
	if !ok {
		t.Fatalf("group[1] = %#v", groups[1])
	}
	if groupJSON0["group_name"] != "production" || groupJSON1["group_id"] != float64(8) {
		t.Fatalf("groups = %#v", groups)
	}
	models, ok := groupJSON0["models"].([]any)
	if !ok || len(models) == 0 {
		t.Fatalf("models = %#v", groupJSON0["models"])
	}
	model, ok := models[0].(map[string]any)
	if !ok {
		t.Fatalf("model = %#v", models[0])
	}
	statusCodes, ok := model["status_codes"].(map[string]any)
	if !ok || statusCodes["500"] != float64(3) {
		t.Fatalf("model = %#v", model)
	}
	accounts, ok := model["accounts"].([]any)
	if !ok || len(accounts) != 2 {
		t.Fatalf("accounts = %#v", model["accounts"])
	}
	backupAccount, ok := accounts[1].(map[string]any)
	if !ok {
		t.Fatalf("account[1] = %#v", accounts[1])
	}
	reasons, ok := backupAccount["reasons"].([]any)
	if !ok || len(reasons) != 2 {
		t.Fatalf("account reasons = %#v", accounts[1])
	}
	reason, ok := reasons[1].(map[string]any)
	if !ok {
		t.Fatalf("reason[1] = %#v", reasons[1])
	}
	if reason["error_type"] != "network_error" || reason["status_code"] != float64(502) || reason["representative_error_id"] != float64(103) {
		t.Fatalf("reason = %#v", reason)
	}
}

func TestOpsUpstreamErrorSummaryNestedEmptyCollectionsAreNotNull(t *testing.T) {
	payload, err := json.Marshal(OpsUpstreamErrorSummary{Groups: []*OpsUpstreamErrorSummaryGroup{{Models: []*OpsUpstreamErrorSummaryModel{{}}}}})
	if err != nil {
		t.Fatalf("marshal nested empty summary: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	group := got["groups"].([]any)[0].(map[string]any)
	model := group["models"].([]any)[0].(map[string]any)
	if group["models"] == nil || model["status_codes"] == nil || model["accounts"] == nil {
		t.Fatalf("nested collection serialized as null: %s", payload)
	}
	if len(model["accounts"].([]any)) != 0 || len(model["status_codes"].(map[string]any)) != 0 {
		t.Fatalf("nested collections not empty: %s", payload)
	}
}
