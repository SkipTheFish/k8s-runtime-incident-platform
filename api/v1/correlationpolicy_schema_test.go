package v1

import (
	"os"
	"testing"

	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

func TestCorrelationPolicyGeneratedSchemaRejectsTypeAndEnumErrors(t *testing.T) {
	payload, err := os.ReadFile("../../config/crd/bases/observability.io_correlationpolicies.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var crd extensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(payload, &crd); err != nil {
		t.Fatal(err)
	}
	spec := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
	enabled := spec.Properties["enabled"]
	if enabled.Type != "boolean" {
		t.Fatalf("enabled type=%q, string values would not be rejected", enabled.Type)
	}
	sources := spec.Properties["sources"]
	if sources.Type != "array" || sources.MinItems == nil || *sources.MinItems != 1 || sources.Items == nil || sources.Items.Schema == nil || len(sources.Items.Schema.Enum) != 2 {
		t.Fatalf("sources constraints missing: %#v", sources)
	}
	severity := spec.Properties["severity"]
	if severity.Type != "string" || len(severity.Enum) != 3 {
		t.Fatalf("severity enum missing: %#v", severity)
	}
	groupBy := spec.Properties["groupBy"]
	if groupBy.MinItems == nil || *groupBy.MinItems != 1 {
		t.Fatalf("groupBy minItems missing: %#v", groupBy)
	}
}
