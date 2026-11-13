package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestUsageLogModelStatsIndexMigration(t *testing.T) {
	data, err := os.ReadFile("265_usage_logs_model_stats_index_notx.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(data))
	for _, want := range []string{
		"create index concurrently if not exists idx_usage_logs_created_model_cost_perf",
		"on usage_logs (created_at, model)",
		"include (total_cost, actual_cost, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens)",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q", want)
		}
	}
}
