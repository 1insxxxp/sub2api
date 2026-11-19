package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

func TestAcceptanceDefaultsForEveryRegisteredPlatform(t *testing.T) {
	for _, platform := range domain.ConcretePlatformIDs() {
		t.Run(platform, func(t *testing.T) {
			a := &Account{ID: 1, Platform: platform, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://relay.invalid"}}
			s := NewChannelAcceptanceService(nil, &acceptanceAccountsStub{account: a}, nil, nil, nil)
			targets, err := s.Targets(context.Background(), []int64{1})
			if err != nil || len(targets) != 1 {
				t.Fatal(targets, err)
			}
			available := false
			for _, model := range targets[0].Models {
				if model.UnavailableReason != "" {
					continue
				}
				if len(model.Protocols) == 0 {
					t.Fatal(model)
				}
				if domain.IsCNProviderPlatform(platform) && strings.HasPrefix(model.Model, "claude-") {
					t.Fatalf("foreign fallback: %s", model.Model)
				}
				available = true
			}
			if !available {
				t.Fatalf("missing default text models for %s", platform)
			}
		})
	}
}

func TestAcceptanceSystemOneDiscovery(t *testing.T) {
	a := &Account{ID: 1, Platform: PlatformTypeSafe, Type: AccountTypeAPIKey}
	s := NewChannelAcceptanceService(nil, &acceptanceAccountsStub{account: a}, nil, nil, nil)
	targets, err := s.Targets(context.Background(), []int64{1})
	if err != nil || len(targets) != 1 || len(targets[0].Models) != 1 {
		t.Fatal(targets, err)
	}
	model := targets[0].Models[0]
	if model.Model != "jev-latest" || !slices.Equal(model.Protocols, []string{"systemone"}) {
		t.Fatal(model)
	}
	for _, invalid := range []string{"gpt-5.4", "jev-unknown"} {
		if _, err := s.targetSnapshot(context.Background(), AcceptanceTargetRequest{AccountID: 1, Model: invalid}); err == nil {
			t.Fatal(invalid)
		}
	}
}

func TestAcceptanceAnthropicMappedTargets6178(t *testing.T) {
	mapping := map[string]any{}
	for index := range 18 {
		mapping[fmt.Sprintf("channel/alias-%02d", index)] = "claude-sonnet-4-6"
	}
	a := &Account{ID: 6178, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": mapping}}
	s := NewChannelAcceptanceService(nil, &acceptanceAccountsStub{account: a}, nil, nil, nil)
	targets, err := s.Targets(context.Background(), []int64{6178})
	if err != nil || len(targets) != 1 || len(targets[0].Models) != 18 {
		t.Fatalf("targets: %+v, %v", targets, err)
	}
	for index, model := range targets[0].Models {
		if model.Model != fmt.Sprintf("channel/alias-%02d", index) || model.UpstreamModel != "claude-sonnet-4-6" || model.Price != nil || len(model.Protocols) == 0 || model.Protocols[0] != "messages" {
			t.Fatalf("unexpected alias: %+v", model)
		}
		snapshot, err := s.targetSnapshot(context.Background(), AcceptanceTargetRequest{AccountID: 6178, Model: model.Model})
		if err != nil || snapshot.AccountType != a.Type || !slices.Equal(snapshot.Protocols, model.Protocols) {
			t.Fatalf("snapshot: %+v, %v", snapshot, err)
		}
	}
}

func TestAcceptanceWildcardAndDedicatedModel(t *testing.T) {
	a := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-*": "gpt-5.4", "picture": "vendor/gpt-image-1"}}}
	s := NewChannelAcceptanceService(nil, &acceptanceAccountsStub{account: a}, nil, nil, nil)
	targets, err := s.Targets(context.Background(), []int64{1})
	if err != nil || len(targets) != 1 {
		t.Fatal(targets, err)
	}
	var chat, picture bool
	for _, m := range targets[0].Models {
		if m.Model == "gpt-*" {
			t.Fatal("wildcard advertised as a callable model")
		}
		if m.Model == "gpt-5.4" {
			chat = len(m.Protocols) > 0
		}
		if m.Model == "picture" {
			picture = m.UnavailableReason != "" && len(m.Protocols) == 0 && m.Price == nil
		}
	}
	if !chat || !picture {
		t.Fatal(targets)
	}
	if _, err := s.targetSnapshot(context.Background(), AcceptanceTargetRequest{AccountID: 1, Model: "picture"}); err == nil {
		t.Fatal("dedicated model allowed through preview")
	}
}
