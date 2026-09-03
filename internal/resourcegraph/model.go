package resourcegraph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ResourceKey is the complete identity of a Kubernetes object. UID deliberately
// remains part of the key so deleting and recreating an object with the same
// namespace and name cannot alias the old graph node.
type ResourceKey struct {
	ClusterID  string `json:"clusterId"`
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
	UID        string `json:"uid"`
}

func (k ResourceKey) Validate() error {
	if strings.TrimSpace(k.ClusterID) == "" || strings.TrimSpace(k.APIVersion) == "" ||
		strings.TrimSpace(k.Kind) == "" || strings.TrimSpace(k.Name) == "" || strings.TrimSpace(k.UID) == "" {
		return errors.New("resource identity requires cluster ID, API version, kind, name, and UID")
	}
	return nil
}

// StableKey returns a compact deterministic representation of the full
// identity. JSON is used as the canonical input to avoid delimiter ambiguity.
func (k ResourceKey) StableKey() (string, error) {
	if err := k.Validate(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(k)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

type ResourceNode struct {
	Key        ResourceKey       `json:"key"`
	Labels     map[string]string `json:"labels,omitempty"`
	ObservedAt time.Time         `json:"observedAt"`
	DeletedAt  *time.Time        `json:"deletedAt,omitempty"`
}

type EdgeType string

const (
	EdgeOwnedBy     EdgeType = "owned_by"
	EdgeScheduledOn EdgeType = "scheduled_on"
)

type ResourceEdge struct {
	ClusterID  string    `json:"clusterId"`
	FromUID    string    `json:"fromUid"`
	ToUID      string    `json:"toUid"`
	Type       EdgeType  `json:"type"`
	ObservedAt time.Time `json:"observedAt"`
}

type RootResolutionResult struct {
	Root ResourceNode   `json:"root"`
	Path []ResourceEdge `json:"path,omitempty"`
}
