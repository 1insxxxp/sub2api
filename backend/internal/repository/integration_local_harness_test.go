//go:build integration

package repository

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	redisclient "github.com/redis/go-redis/v9"
)

// Explicit loopback test databases allow the same real-PostgreSQL integration
// suite to run on developer machines without a working Docker daemon. Never
// accept a production DSN, default application database or remote Redis here.
func runLocalIntegrationHarness(m *testing.M, dsn, redisAddr string) int {
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() != "127.0.0.1" || !strings.HasSuffix(u.Path, "_test") {
		log.Printf("local integration harness requires an explicit loopback PostgreSQL URL and a database ending in _test")
		return 1
	}
	host, port, err := net.SplitHostPort(redisAddr)
	if err != nil || host != "127.0.0.1" || port == "6379" {
		log.Printf("local integration harness requires a dedicated loopback Redis test port")
		return 1
	}
	ctx := context.Background()
	integrationDB, err = openSQLWithRetry(ctx, dsn, 10*time.Second)
	if err != nil {
		log.Printf("open local integration database: %v", err)
		return 1
	}
	defer integrationDB.Close()
	if err := ApplyMigrations(ctx, integrationDB); err != nil {
		log.Printf("apply local integration migrations: %v", err)
		return 1
	}
	integrationPostgresDSN = dsn
	integrationEntClient = dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, integrationDB)))
	defer integrationEntClient.Close()
	integrationRedis = redisclient.NewClient(&redisclient.Options{Addr: redisAddr})
	defer integrationRedis.Close()
	if err := integrationRedis.Ping(ctx).Err(); err != nil {
		log.Printf("local integration Redis: %v", err)
		return 1
	}
	fmt.Println("integration harness: dedicated local PostgreSQL and Redis (Docker bypassed)")
	return m.Run()
}
