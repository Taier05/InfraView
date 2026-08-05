package rabbitmq_test

import (
	"strings"
	"testing"

	"github.com/Taier05/InfraView/internal/rabbitmq"
	"github.com/Taier05/InfraView/internal/rabbitmq/rabbitmqtest"
)

func TestStableIDsSeparateClustersAndNodes(t *testing.T) {
	first := rabbitmq.StableNodeID("fixture-cluster-a", "fixture-node-a")
	if first == "" || first == rabbitmq.StableNodeID("fixture-cluster-b", "fixture-node-a") {
		t.Fatal("node IDs must be non-empty and cluster scoped")
	}
	if strings.Contains(first, "fixture-cluster-a") || strings.Contains(first, "fixture-node-a") {
		t.Fatal("stable ID exposed raw identity")
	}
}

func TestSnapshotCloneDoesNotAliasPointers(t *testing.T) {
	snapshot := rabbitmqtest.Snapshot()
	clone := snapshot.Clone()
	*clone.Nodes[0].Connections = 999
	if *snapshot.Nodes[0].Connections == 999 {
		t.Fatal("clone aliases source")
	}
}

func TestSnapshotClonePreservesNilSlices(t *testing.T) {
	clone := (rabbitmq.Snapshot{Clusters: nil, Nodes: nil}).Clone()
	if clone.Clusters != nil || clone.Nodes != nil {
		t.Fatal("clone must preserve nil slices")
	}
}

func TestSnapshotClonePreservesNonNilEmptySlices(t *testing.T) {
	clone := (rabbitmq.Snapshot{Clusters: []rabbitmq.Cluster{}, Nodes: []rabbitmq.Node{}}).Clone()
	if clone.Clusters == nil || clone.Nodes == nil {
		t.Fatal("clone must preserve non-nil empty slices")
	}
	if len(clone.Clusters) != 0 || len(clone.Nodes) != 0 {
		t.Fatal("clone changed empty slice lengths")
	}
}
