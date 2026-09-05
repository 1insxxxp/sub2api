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
		mac.Write(body)
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
