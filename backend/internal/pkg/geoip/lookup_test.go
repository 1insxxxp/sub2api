//go:build unit

package geoip

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLookupRejectsInvalidInputBeforeOpeningDatabase(t *testing.T) {
	db := New(filepath.Join(t.TempDir(), "missing-private-database.mmdb"))
	for _, ips := range [][]string{nil, {}, {"not-an-ip"}, {"8.8.8.8:443"}, {"fe80::1%en0"}, make([]string, 51)} {
		_, err := db.Lookup(ips)
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("want invalid input, got %v", err)
		}
	}
}

func TestLookupPrivateAddressesDoesNotNeedDatabase(t *testing.T) {
	db := New(filepath.Join(t.TempDir(), "missing.mmdb"))
	ips := []string{"10.0.0.1", "127.0.0.1", "172.16.1.1", "192.168.0.1", "169.254.0.1", "::1", "fd00::1", "fe80::1", "::ffff:192.168.1.1", "0.0.0.0", "224.0.0.1"}
	results, err := db.Lookup(ips)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(ips) {
		t.Fatalf("want %d results, got %d", len(ips), len(results))
	}
	for i, result := range results {
		if result.IP != ips[i] || !result.Private || result.CountryCode != "" {
			t.Fatalf("unexpected private result: %+v", result)
		}
	}
}

func TestLookupMissingOrInvalidDatabaseHasGenericError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sensitive-private-path.mmdb")
	for _, content := range [][]byte{nil, []byte("invalid database")} {
		if content != nil {
			if err := os.WriteFile(path, content, 0600); err != nil {
				t.Fatal(err)
			}
		}
		_, err := New(path).Lookup([]string{"8.8.8.8"})
		if err != ErrUnavailable {
			t.Fatalf("want generic unavailable, got %v", err)
		}
	}
}

func TestDefaultPathUsesConfiguredDatabaseThenDataDirectory(t *testing.T) {
	t.Setenv("GEOIP_DATABASE_PATH", "")
	t.Setenv("DATA_DIR", "")
	if got := DatabasePath(); got != filepath.Join("data", "geoip.mmdb") {
		t.Fatal(got)
	}
	t.Setenv("DATA_DIR", "/private-data")
	if got := DatabasePath(); got != "/private-data/geoip.mmdb" {
		t.Fatal(got)
	}
	t.Setenv("GEOIP_DATABASE_PATH", "/offline/custom.mmdb")
	if got := DatabasePath(); got != "/offline/custom.mmdb" {
		t.Fatal(got)
	}
}

func TestPrivateClassificationUnmapsIPv4(t *testing.T) {
	if !isPrivate(netip.MustParseAddr("::ffff:10.0.0.1")) {
		t.Fatal("mapped private address should be private")
	}
	if isPrivate(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("public address should not be private")
	}
}

func TestDecodeCityRecordKeepsOnlyLocationFields(t *testing.T) {
	record := cityRecord{}
	record.Country.ISOCode = "GB"
	record.City.Names = map[string]string{"en": "London"}
	record.Subdivisions = []namedPlace{{Names: map[string]string{"en": "England"}}}
	lat, lon := 51.5142, -0.0931
	record.Location.Latitude, record.Location.Longitude = &lat, &lon
	record.Location.TimeZone = "Europe/London"
	result := record.result("81.2.69.142", "dbip")
	if result.CountryCode != "GB" || result.Region != "England" || result.City != "London" || result.Timezone != "Europe/London" || result.Latitude != "51.5142" || result.Longitude != "-0.0931" || result.Source != "dbip" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestLocalMMDBFixture(t *testing.T) {
	path := filepath.Join("testdata", "GeoIP2-City-Test.mmdb")
	db := New(path)
	results, err := db.Lookup([]string{"81.2.69.142", "::ffff:81.2.69.142", "1.1.1.1"})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if results[2].CountryCode != "" {
		t.Fatalf("unexpected location for unknown test address: %+v", results[2])
	}
	for _, result := range results[:2] {
		if result.CountryCode != "GB" || result.City != "London" || result.Timezone != "Europe/London" {
			t.Fatalf("unexpected fixture result: %+v", result)
		}
	}
}

func TestInstalledCityDatabaseSmoke(t *testing.T) {
	path := os.Getenv("GEOIP_SMOKE_DATABASE_PATH")
	if path == "" {
		t.Skip("optional smoke check for an operator-installed city database")
	}
	db := New(path)
	defer db.Close()
	results, err := db.Lookup([]string{"8.8.8.8", "2001:4860:4860::8888", "192.168.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].CountryCode != "US" || results[0].City == "" || results[0].Source != "dbip" {
		t.Fatalf("unexpected DB-IP record: %+v", results[0])
	}
	if results[1].CountryCode == "" || !results[2].Private {
		t.Fatalf("IPv6/private lookup failed: %+v", results)
	}
}

func TestDatabaseConcurrentLookupsAndClose(t *testing.T) {
	db := New(filepath.Join("testdata", "GeoIP2-City-Test.mmdb"))
	if _, err := db.Lookup([]string{"81.2.69.142"}); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 20 {
				results, err := db.Lookup([]string{"81.2.69.142"})
				if err != nil && !errors.Is(err, ErrUnavailable) {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if err == nil && results[0].CountryCode != "GB" {
					t.Error("incorrect concurrent result")
					return
				}
			}
		}()
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	workers.Wait()
	if _, err := db.Lookup([]string{"81.2.69.142"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed database returned %v", err)
	}
}
