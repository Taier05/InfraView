package elasticsearchtest

import (
	"context"
	"reflect"
	"testing"

	"github.com/Taier05/InfraView/internal/elasticsearch"
)

func RunContract(t testing.TB, provider elasticsearch.Provider) {
	t.Helper()
	first, err := provider.ElasticsearchSnapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	second, err := provider.ElasticsearchSnapshot(context.Background())
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if len(first.Clusters) == 0 || len(first.Nodes) == 0 {
		t.Fatal("contract requires clusters and nodes")
	}
	if len(first.Nodes[0].Roles) == 0 {
		t.Fatal("contract requires node roles")
	}
	first.Nodes[0].Roles[0] = elasticsearch.RoleIngest
	if reflect.DeepEqual(first, second) {
		t.Fatal("provider returned shared mutable state")
	}
}
