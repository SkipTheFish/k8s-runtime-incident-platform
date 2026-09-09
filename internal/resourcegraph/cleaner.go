package resourcegraph

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) CleanExpired(ctx context.Context, cutoff time.Time, batch int, transactionTimeout time.Duration) (int64, error) {
	if batch < 1 {
		return 0, fmt.Errorf("cleanup batch must be positive")
	}
	txCtx, cancel := context.WithTimeout(ctx, transactionTimeout)
	defer cancel()
	tx, err := r.db.BeginTx(txCtx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(txCtx)
	rows, err := tx.Query(txCtx, `SELECT r.cluster_id,r.uid FROM runtime_resources r
WHERE r.deleted_at IS NOT NULL AND r.deleted_at <= $1
AND NOT EXISTS (SELECT 1 FROM signals s WHERE s.cluster_id=r.cluster_id AND s.root_resource_uid=r.uid)
AND NOT EXISTS (SELECT 1 FROM incidents i WHERE i.root_resource->>'uid'=r.uid)
AND NOT EXISTS (SELECT 1 FROM correlation_decisions c WHERE c.root_resource->>'uid'=r.uid)
AND NOT EXISTS (SELECT 1 FROM timeline_entries t WHERE jsonb_path_exists(t.evidence, '$.** ? (@ == $uid)', jsonb_build_object('uid', r.uid)))
ORDER BY r.deleted_at,r.id FOR UPDATE SKIP LOCKED LIMIT $2`, cutoff, batch)
	if err != nil {
		return 0, fmt.Errorf("select expired resources: %w", err)
	}
	var clusters, uids []string
	for rows.Next() {
		var cluster, uid string
		if err := rows.Scan(&cluster, &uid); err != nil {
			rows.Close()
			return 0, err
		}
		clusters = append(clusters, cluster)
		uids = append(uids, uid)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	if len(uids) == 0 {
		if err := tx.Commit(txCtx); err != nil {
			return 0, err
		}
		return 0, nil
	}
	if _, err := tx.Exec(txCtx, `DELETE FROM resource_edges e USING unnest($1::text[],$2::text[]) x(cluster_id,uid) WHERE e.cluster_id=x.cluster_id AND (e.from_uid=x.uid OR e.to_uid=x.uid)`, clusters, uids); err != nil {
		return 0, fmt.Errorf("delete expired resource edges: %w", err)
	}
	tag, err := tx.Exec(txCtx, `DELETE FROM runtime_resources r USING unnest($1::text[],$2::text[]) x(cluster_id,uid) WHERE r.cluster_id=x.cluster_id AND r.uid=x.uid`, clusters, uids)
	if err != nil {
		return 0, fmt.Errorf("delete expired resources: %w", err)
	}
	if err := tx.Commit(txCtx); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

type CleanupFunc func(context.Context, time.Time, int, time.Duration) (int64, error)

type Cleaner struct {
	retention time.Duration
	interval  time.Duration
	timeout   time.Duration
	batch     int
	clean     CleanupFunc
	now       func() time.Time
}

func NewCleaner(retention, interval, timeout time.Duration, batch int, clean CleanupFunc) *Cleaner {
	return &Cleaner{retention: retention, interval: interval, timeout: timeout, batch: batch, clean: clean, now: time.Now}
}

func (c *Cleaner) Run(ctx context.Context) error {
	if err := c.runOnce(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.runOnce(ctx); err != nil {
				return err
			}
		}
	}
}

func (c *Cleaner) runOnce(ctx context.Context) error {
	_, err := c.clean(ctx, c.now().Add(-c.retention), c.batch, c.timeout)
	return err
}
