package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestUsageLogRankingIndexMigration(t *testing.T) {
	data, err := os.ReadFile("264_usage_logs_ranking_index_notx.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(data))
	for _, want := range []string{
		"create index concurrently if not exists idx_usage_logs_billing_created_user_perf",
		"on usage_logs (billing_type, created_at, user_id)",
		"include (actual_cost, compensated_cost, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens)",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q", want)
		}
	}
}
