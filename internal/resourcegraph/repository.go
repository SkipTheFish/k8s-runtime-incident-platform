package resourcegraph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) Apply(ctx context.Context, task ResourceTask) error {
	if err := task.Node.Key.Validate(); err != nil {
		return fmt.Errorf("validate resource task: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	key := task.Node.Key
	if task.Action == TaskDelete {
		if _, err := tx.Exec(ctx, `UPDATE runtime_resources SET deleted_at=$3,observed_at=GREATEST(observed_at,$3),updated_at=now() WHERE cluster_id=$1 AND uid=$2`, key.ClusterID, key.UID, task.Node.ObservedAt); err != nil {
			return fmt.Errorf("soft delete resource: %w", err)
		}
		return tx.Commit(ctx)
	}
	labels, err := json.Marshal(task.Node.Labels)
	if err != nil {
		return fmt.Errorf("encode resource labels: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO runtime_resources(cluster_id,uid,api_version,kind,namespace,name,labels,observed_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT(cluster_id,uid) DO UPDATE SET api_version=EXCLUDED.api_version,kind=EXCLUDED.kind,namespace=EXCLUDED.namespace,name=EXCLUDED.name,labels=EXCLUDED.labels,observed_at=GREATEST(runtime_resources.observed_at,EXCLUDED.observed_at),deleted_at=NULL,updated_at=now()`,
		key.ClusterID, key.UID, key.APIVersion, key.Kind, key.Namespace, key.Name, labels, task.Node.ObservedAt); err != nil {
		return fmt.Errorf("upsert resource: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM resource_edges WHERE cluster_id=$1 AND from_uid=$2 AND edge_type IN ($3,$4)`, key.ClusterID, key.UID, EdgeOwnedBy, EdgeScheduledOn); err != nil {
		return fmt.Errorf("replace resource edges: %w", err)
	}
	for _, ownerUID := range task.OwnerUIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO resource_edges(cluster_id,from_uid,to_uid,edge_type,observed_at)
SELECT $1,$2,$3,$4,$5 WHERE EXISTS(SELECT 1 FROM runtime_resources WHERE cluster_id=$1 AND uid=$3)
ON CONFLICT(cluster_id,from_uid,to_uid,edge_type) DO UPDATE SET observed_at=GREATEST(resource_edges.observed_at,EXCLUDED.observed_at),updated_at=now()`, key.ClusterID, key.UID, ownerUID, EdgeOwnedBy, task.Node.ObservedAt); err != nil {
			return fmt.Errorf("upsert owner edge: %w", err)
		}
	}
	if key.Kind == "Pod" && task.NodeName != "" {
		if _, err := tx.Exec(ctx, `INSERT INTO resource_edges(cluster_id,from_uid,to_uid,edge_type,observed_at)
SELECT $1,$2,uid,$3,$4 FROM runtime_resources WHERE cluster_id=$1 AND kind='Node' AND namespace='' AND name=$5 AND deleted_at IS NULL
ON CONFLICT(cluster_id,from_uid,to_uid,edge_type) DO UPDATE SET observed_at=GREATEST(resource_edges.observed_at,EXCLUDED.observed_at),updated_at=now()`, key.ClusterID, key.UID, EdgeScheduledOn, task.Node.ObservedAt, task.NodeName); err != nil {
			return fmt.Errorf("upsert node edge: %w", err)
		}
	}
	return tx.Commit(ctx)
}
