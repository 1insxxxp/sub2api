package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCatalogSnapshotPusherSignsAndSends(t *testing.T) {
	secret := []byte("test-secret")
	type incoming struct {
		r    *http.Request
		body []byte
	}
	received := make(chan incoming, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- incoming{r: r, body: body}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	p := &CatalogSnapshotPusher{Endpoint: srv.URL, Secret: secret, Build: func(context.Context) (CatalogSnapshot, error) {
		return CatalogSnapshot{Source: "sub2api", Version: 1}, nil
	}}
	p.PushAsync()
	select {
	case got := <-received:
		r, body := got.r, got.body
		mac := hmac.New(sha256.New, secret)
		_, _ = mac.Write(body)
		if got, want := r.Header.Get("X-Catalog-Signature"), hex.EncodeToString(mac.Sum(nil)); got != want {
			t.Fatalf("signature=%s want %s", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("push did not arrive")
	}
}

func TestShouldPushForSchedulerEvent(t *testing.T) {
	if ShouldPushForSchedulerEvent(SchedulerOutboxEventAccountLastUsed) {
		t.Fatal("last-used should be ignored")
	}
	if ShouldPushForSchedulerEvent("unknown") {
		t.Fatal("unknown event should be ignored")
	}
}

func TestValidateCatalogSnapshot(t *testing.T) {
	if err := ValidateCatalogSnapshot(CatalogSnapshot{Source: "other", Version: 1}); err == nil {
		t.Fatal("expected source validation")
	}
	if err := ValidateCatalogSnapshot(CatalogSnapshot{Source: "sub2api", Version: 1, GeneratedAt: "2026-09-05T00:00:00Z", Groups: []CatalogGroup{{ID: 1, Name: "g", Platform: PlatformOpenAI, Enabled: true, Models: []CatalogModel{{Name: "m", Platform: PlatformOpenAI, Enabled: true}}}}}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCatalogSnapshotRejectsUnsafeVersionAndNullGroups(t *testing.T) {
	base := CatalogSnapshot{Source: "sub2api", Version: 1, GeneratedAt: "2026-09-05T00:00:00Z", Groups: []CatalogGroup{}}
	base.Version = 1 << 53
	if err := ValidateCatalogSnapshot(base); err == nil {
		t.Fatal("expected unsafe version rejection")
	}
	base.Version = 1
	base.Groups = nil
	if err := ValidateCatalogSnapshot(base); err == nil {
		t.Fatal("expected null groups rejection")
	}
}

func TestCatalogModelsForGroupMirrorsGatewayModelAvailability(t *testing.T) {
	group := Group{Platform: PlatformOpenAI, ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"gpt-5.4", "not-a-provider-model"}}}
	accounts := []Account{{Platform: PlatformOpenAI, Extra: map[string]any{"openai_passthrough": true}}}
	models := catalogModelsForGroup(group, accounts)
	if len(models) != 1 || models[0].Name != "gpt-5.4" {
		t.Fatalf("models=%#v, want only provider default gpt-5.4", models)
	}
}

type catalogGroupRepoStub struct {
	GroupRepository
	groups []Group
}

func (s *catalogGroupRepoStub) ListActive(context.Context) ([]Group, error) { return s.groups, nil }

type catalogAccountRepoStub struct {
	AccountRepository
	accounts []Account
}

func (s *catalogAccountRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	return s.accounts, nil
}

func TestBuildCatalogSnapshotPreservesGroupSortOrder(t *testing.T) {
	groups := []Group{
		{ID: 10, Name: "第二组", Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 20},
		{ID: 20, Name: "第一组", Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 10},
	}
	accounts := []Account{{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}}
	build := buildCatalogSnapshot(&catalogGroupRepoStub{groups: groups}, &catalogAccountRepoStub{accounts: accounts})
	snapshot, err := build(context.Background())
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	if len(snapshot.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(snapshot.Groups))
	}
	if got := snapshot.Groups[0].ID; got != 10 {
		t.Fatalf("expected repository order to be preserved, got first group %d", got)
	}
	if snapshot.Groups[0].SortOrder != 20 || snapshot.Groups[1].SortOrder != 10 {
		t.Fatalf("unexpected sort orders: %+v", snapshot.Groups)
	}
}
