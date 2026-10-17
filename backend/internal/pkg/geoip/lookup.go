// Package geoip resolves addresses exclusively from a locally installed MMDB.
// It never sends a queried address to an external geolocation provider.
package geoip

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/oschwald/maxminddb-golang/v2"
)

var (
	ErrInvalidInput = errors.New("provide between 1 and 50 valid IP addresses")
	ErrUnavailable  = errors.New("offline IP geolocation is unavailable")
	defaultOnce     sync.Once
	defaultDatabase *Database
)

// Result contains presentation data only; internal database paths are never returned.
type Result struct {
	IP           string  `json:"ip"`
	Private      bool    `json:"private,omitempty"`
	CountryCode  string  `json:"country_code,omitempty"`
	Region       string  `json:"region,omitempty"`
	City         string  `json:"city,omitempty"`
	Organization string  `json:"organization,omitempty"`
	Timezone     string  `json:"timezone,omitempty"`
	Accuracy     *uint16 `json:"accuracy,omitempty"`
	Latitude     string  `json:"latitude,omitempty"`
	Longitude    string  `json:"longitude,omitempty"`
	Source       string  `json:"source,omitempty"`
}

type query struct {
	original string
	address  netip.Addr
}

// Database shares a read-only MMDB across concurrent requests. Replace the file
// atomically and restart the application to load a new database revision.
type Database struct {
	path   string
	once   sync.Once
	mu     sync.RWMutex
	reader *maxminddb.Reader
	closed bool
}

func New(path string) *Database { return &Database{path: path} }

func DatabasePath() string {
	if path := strings.TrimSpace(os.Getenv("GEOIP_DATABASE_PATH")); path != "" {
		return path
	}
	dir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dir == "" {
		dir = "data"
	}
	return filepath.Join(dir, "geoip.mmdb")
}

func Lookup(ips []string) ([]Result, error) {
	defaultOnce.Do(func() { defaultDatabase = New(DatabasePath()) })
	return defaultDatabase.Lookup(ips)
}

func ValidateIPs(ips []string) error {
	_, err := parseQueries(ips)
	return err
}

func parseQueries(ips []string) ([]query, error) {
	if len(ips) < 1 || len(ips) > 50 {
		return nil, ErrInvalidInput
	}
	out := make([]query, 0, len(ips))
	for _, ip := range ips {
		address, err := netip.ParseAddr(ip)
		if err != nil || address.Zone() != "" {
			return nil, ErrInvalidInput
		}
		out = append(out, query{original: ip, address: address.Unmap()})
	}
	return out, nil
}

func isPrivate(address netip.Addr) bool {
	address = address.Unmap()
	return !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLinkLocalUnicast()
}

func onlyPrivate(queries []query) bool {
	for _, q := range queries {
		if !isPrivate(q.address) {
			return false
		}
	}
	return true
}

func (db *Database) Lookup(ips []string) ([]Result, error) {
	queries, err := parseQueries(ips)
	if err != nil {
		return nil, err
	}
	if onlyPrivate(queries) {
		return privateResults(queries), nil
	}
	db.mu.RLock()
	defer db.mu.RUnlock()
	if db.closed {
		return nil, ErrUnavailable
	}
	db.once.Do(func() { db.reader, _ = maxminddb.Open(db.path) })
	if db.reader == nil {
		return nil, ErrUnavailable
	}
	return db.lookupRecords(queries)
}

func privateResults(queries []query) []Result {
	results := make([]Result, 0, len(queries))
	for _, q := range queries {
		results = append(results, Result{IP: q.original, Private: true})
	}
	return results
}

func (db *Database) lookupRecords(queries []query) ([]Result, error) {
	results := make([]Result, 0, len(queries))
	source := databaseSource(db.reader.Metadata.DatabaseType)
	for _, q := range queries {
		if isPrivate(q.address) {
			results = append(results, Result{IP: q.original, Private: true})
			continue
		}
		var record cityRecord
		if err := db.reader.Lookup(q.address).Decode(&record); err != nil {
			return nil, ErrUnavailable
		}
		results = append(results, record.result(q.original, source))
	}
	return results, nil
}

func (db *Database) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.closed = true
	if db.reader != nil {
		err := db.reader.Close()
		db.reader = nil
		return err
	}
	return nil
}

func databaseSource(databaseType string) string {
	name := strings.ToLower(databaseType)
	if strings.Contains(name, "dbip") {
		return "dbip"
	}
	if strings.Contains(name, "geoip") || strings.Contains(name, "geolite") {
		return "maxmind"
	}
	return ""
}

type namedPlace struct {
	Names map[string]string `maxminddb:"names"`
}
type cityRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	City            namedPlace   `maxminddb:"city"`
	Subdivisions    []namedPlace `maxminddb:"subdivisions,maxsize:32"`
	Organization    string       `maxminddb:"organization"`
	ASNOrganization string       `maxminddb:"autonomous_system_organization"`
	Location        struct {
		TimeZone       string   `maxminddb:"time_zone"`
		AccuracyRadius *uint16  `maxminddb:"accuracy_radius"`
		Latitude       *float64 `maxminddb:"latitude"`
		Longitude      *float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

func (record cityRecord) result(ip, source string) Result {
	result := Result{IP: ip, CountryCode: record.Country.ISOCode, City: record.City.Names["en"],
		Timezone: record.Location.TimeZone, Accuracy: record.Location.AccuracyRadius,
		Organization: record.Organization, Source: source}
	if len(record.Subdivisions) > 0 {
		result.Region = record.Subdivisions[0].Names["en"]
	}
	if result.Organization == "" {
		result.Organization = record.ASNOrganization
	}
	if record.Location.Latitude != nil {
		result.Latitude = strconv.FormatFloat(*record.Location.Latitude, 'f', -1, 64)
	}
	if record.Location.Longitude != nil {
		result.Longitude = strconv.FormatFloat(*record.Location.Longitude, 'f', -1, 64)
	}
	return result
}
