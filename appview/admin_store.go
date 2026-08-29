package main

import (
	"context"
	"encoding/json"
	"time"
)

type AdminRelationSize struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

type AdminDatabaseMetrics struct {
	SizeBytes       int64               `json:"sizeBytes"`
	ConnectionCount int                 `json:"connectionCount"`
	MaxConnections  int                 `json:"maxConnections"`
	PendingReview   int64               `json:"pendingReview"`
	LargestTables   []AdminRelationSize `json:"largestTables"`
}

type AdminPoolMetrics struct {
	AcquiredConns       int32 `json:"acquiredConns"`
	IdleConns           int32 `json:"idleConns"`
	TotalConns          int32 `json:"totalConns"`
	MaxConns            int32 `json:"maxConns"`
	EmptyAcquireCount   int64 `json:"emptyAcquireCount"`
	AcquireDurationMSec int64 `json:"acquireDurationMs"`
}

type OperationsJobRun struct {
	Job        string          `json:"job"`
	Status     string          `json:"status"`
	StartedAt  time.Time       `json:"startedAt"`
	FinishedAt time.Time       `json:"finishedAt"`
	Details    json.RawMessage `json:"details"`
}

type OperationsHostSnapshot struct {
	Host       string          `json:"host"`
	ReportedAt time.Time       `json:"reportedAt"`
	Payload    json.RawMessage `json:"payload"`
}

func (m *PgStore) IsAdmin(ctx context.Context, did string) (bool, error) {
	var found bool
	err := m.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM admin WHERE did = $1 AND disabled_at IS NULL)
	`, did).Scan(&found)
	return found, err
}

func (m *PgStore) AdminDatabaseMetrics(ctx context.Context) (AdminDatabaseMetrics, error) {
	var metrics AdminDatabaseMetrics
	err := m.pool.QueryRow(ctx, `
		SELECT
			pg_database_size(current_database()),
			(SELECT count(*) FROM pg_stat_activity WHERE datname = current_database()),
			current_setting('max_connections')::int,
			(SELECT count(*) FROM review_item WHERE status = 'pending')
	`).Scan(&metrics.SizeBytes, &metrics.ConnectionCount, &metrics.MaxConnections, &metrics.PendingReview)
	if err != nil {
		return AdminDatabaseMetrics{}, err
	}

	rows, err := m.pool.Query(ctx, `
		SELECT c.relname, pg_total_relation_size(c.oid)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r'
		ORDER BY pg_total_relation_size(c.oid) DESC
		LIMIT 5
	`)
	if err != nil {
		return AdminDatabaseMetrics{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var table AdminRelationSize
		if err := rows.Scan(&table.Name, &table.Bytes); err != nil {
			return AdminDatabaseMetrics{}, err
		}
		metrics.LargestTables = append(metrics.LargestTables, table)
	}
	if err := rows.Err(); err != nil {
		return AdminDatabaseMetrics{}, err
	}
	return metrics, nil
}

func (m *PgStore) AdminPoolMetrics() AdminPoolMetrics {
	stats := m.pool.Stat()
	return AdminPoolMetrics{
		AcquiredConns:       stats.AcquiredConns(),
		IdleConns:           stats.IdleConns(),
		TotalConns:          stats.TotalConns(),
		MaxConns:            stats.MaxConns(),
		EmptyAcquireCount:   stats.EmptyAcquireCount(),
		AcquireDurationMSec: stats.AcquireDuration().Milliseconds(),
	}
}

func (m *PgStore) LatestOperationsJobRuns(ctx context.Context) ([]OperationsJobRun, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT DISTINCT ON (job) job, status, started_at, finished_at, details
		FROM operations_job_run
		ORDER BY job, finished_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []OperationsJobRun
	for rows.Next() {
		var run OperationsJobRun
		if err := rows.Scan(&run.Job, &run.Status, &run.StartedAt, &run.FinishedAt, &run.Details); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (m *PgStore) LatestOperationsHostSnapshots(ctx context.Context) ([]OperationsHostSnapshot, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT host, reported_at, payload
		FROM operations_host_snapshot
		ORDER BY host
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []OperationsHostSnapshot
	for rows.Next() {
		var snapshot OperationsHostSnapshot
		if err := rows.Scan(&snapshot.Host, &snapshot.ReportedAt, &snapshot.Payload); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, rows.Err()
}

func (m *PgStore) UpsertOperationsHostSnapshot(ctx context.Context, host string, payload json.RawMessage) error {
	_, err := m.pool.Exec(ctx, `
		INSERT INTO operations_host_snapshot (host, payload)
		VALUES ($1, $2)
		ON CONFLICT (host) DO UPDATE SET reported_at = now(), payload = EXCLUDED.payload
	`, host, payload)
	return err
}
