package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
)

func TestNormalizeCustomMenuOpenMode(t *testing.T) {
	tests := []struct {
		name     string
		item     dto.CustomMenuItem
		wantMode string
		wantErr  bool
	}{
		{
			name:     "legacy empty mode defaults to embedded",
			item:     dto.CustomMenuItem{URL: "https://example.com"},
			wantMode: "embedded",
		},
		{
			name:     "embedded is accepted",
			item:     dto.CustomMenuItem{URL: "https://example.com", OpenMode: "embedded"},
			wantMode: "embedded",
		},
		{
			name:     "new tab is accepted",
			item:     dto.CustomMenuItem{URL: "https://example.com", OpenMode: "new_tab"},
			wantMode: "new_tab",
		},
		{
			name:    "unknown mode is rejected",
			item:    dto.CustomMenuItem{URL: "https://example.com", OpenMode: "popup"},
			wantErr: true,
		},
		{
			name:    "markdown cannot open in new tab",
			item:    dto.CustomMenuItem{URL: "md:help", OpenMode: "new_tab"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := tt.item
			err := normalizeCustomMenuOpenMode(&item)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeCustomMenuOpenMode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && item.OpenMode != tt.wantMode {
				t.Fatalf("OpenMode = %q, want %q", item.OpenMode, tt.wantMode)
			}
		})
	}
}
