package observability_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ruhuang/ink/server/internal/platform/observability"
)

func TestPoolMetricsWithoutDatabase(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://test:test@127.0.0.1:1/test?pool_max_conns=7&pool_min_conns=0&sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	m := observability.NewMetrics()
	m.RegisterPool(pool)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if conn, err := pool.Acquire(ctx); err == nil {
		conn.Release()
		t.Fatal("canceled acquire unexpectedly succeeded")
	}
	for range 2 {
		body := scrape(t, m)
		for name, want := range map[string]float64{
			"max_connections": 7, "connections": 0, "acquired_connections": 0,
			"idle_connections": 0, "constructing_connections": 0,
			"acquires_total": 0, "empty_acquires_total": 0,
			"canceled_acquires_total": 1, "acquire_duration_seconds_total": 0,
		} {
			if got := sample(t, body, "ink_db_pool_"+name); got != want {
				t.Errorf("%s: got %v, want %v", name, got, want)
			}
		}
	}
}
