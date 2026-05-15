package graph

import (
	"context"
	"fmt"

	"observability-platform/internal/repository"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func NodeKey(kind, namespace, name string) string {
	return fmt.Sprintf("%s:%s/%s", kind, namespace, name)
}

func (r *Repository) UpsertNode(
	ctx context.Context,
	kind string,
	namespace string,
	name string,
	tenantID string,
) error {
	key := NodeKey(kind, namespace, name)

	_, err := repository.DB.Exec(ctx, `
		INSERT INTO runtime_graph_nodes
		(node_key, kind, namespace, name, tenant_id, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (node_key)
		DO UPDATE SET
		    tenant_id = EXCLUDED.tenant_id,
		    updated_at = NOW()
	`,
		key,
		kind,
		namespace,
		name,
		tenantID,
	)

	return err
}

func (r *Repository) UpsertEdge(
	ctx context.Context,
	fromKind string,
	fromNamespace string,
	fromName string,
	toKind string,
	toNamespace string,
	toName string,
	edgeType string,
) error {
	fromKey := NodeKey(fromKind, fromNamespace, fromName)
	toKey := NodeKey(toKind, toNamespace, toName)

	_, err := repository.DB.Exec(ctx, `
		INSERT INTO runtime_graph_edges
		(from_key, to_key, edge_type, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (from_key, to_key, edge_type)
		DO UPDATE SET updated_at = NOW()
	`,
		fromKey,
		toKey,
		edgeType,
	)

	return err
}

func (r *Repository) FindPoliciesRelatedToResource(
	ctx context.Context,
	namespace string,
	resourceKind string,
	resourceName string,
) ([]string, error) {
	toKey := NodeKey(resourceKind, namespace, resourceName)

	rows, err := repository.DB.Query(ctx, `
		SELECT from_key
		FROM runtime_graph_edges
		WHERE to_key = $1
		  AND edge_type IN ('correlates_with', 'watches', 'targets')
	`, toKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string

	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		result = append(result, key)
	}

	return result, rows.Err()
}