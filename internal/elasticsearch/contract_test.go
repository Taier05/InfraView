package elasticsearch

import "testing"

func TestStableIDsUseOnlyDomainIdentity(t *testing.T) {
	clusterID := StableClusterID("fixture-cluster-a")
	if clusterID == "" || clusterID != StableClusterID("fixture-cluster-a") {
		t.Fatal("cluster ID must be deterministic")
	}
	if clusterID == StableClusterID("fixture-cluster-b") {
		t.Fatal("different clusters must have different IDs")
	}
	nodeID := StableNodeID("fixture-cluster-a", "fixture-node-a")
	if nodeID == "" || nodeID != StableNodeID("fixture-cluster-a", "fixture-node-a") {
		t.Fatal("node ID must be deterministic")
	}
}

func TestStableIDsNormalizeWhitespaceAndRejectMissingIdentity(t *testing.T) {
	if got, want := StableClusterID(" fixture-cluster-a "), StableClusterID("fixture-cluster-a"); got != want || got == "" {
		t.Fatalf("StableClusterID() = %q, want normalized non-empty ID %q", got, want)
	}
	if StableClusterID("") != "" || StableClusterID(" \t\n ") != "" {
		t.Fatal("StableClusterID must reject missing cluster identity")
	}
	if got, want := StableNodeID(" fixture-cluster-a ", " fixture-node-a "), StableNodeID("fixture-cluster-a", "fixture-node-a"); got != want || got == "" {
		t.Fatalf("StableNodeID() = %q, want normalized non-empty ID %q", got, want)
	}
	if StableNodeID("", "fixture-node-a") != "" || StableNodeID("fixture-cluster-a", " \t\n ") != "" {
		t.Fatal("StableNodeID must reject missing cluster or node identity")
	}
	if StableClusterID("fixture-cluster-a") == StableNodeID("fixture-cluster-a", "") {
		t.Fatal("stable IDs must keep domain identities separate")
	}
}

func TestSnapshotCloneDeepCopiesRolesAndPointers(t *testing.T) {
	original := Snapshot{Nodes: []Node{{
		ID:            StableNodeID("fixture-cluster-a", "fixture-node-a"),
		Roles:         []Role{RoleMaster, RoleData},
		HeapUsedBytes: int64Pointer(40),
	}}}
	clone := original.Clone()
	clone.Nodes[0].Roles[0] = RoleIngest
	*clone.Nodes[0].HeapUsedBytes = 90
	if original.Nodes[0].Roles[0] != RoleMaster || *original.Nodes[0].HeapUsedBytes != 40 {
		t.Fatal("clone mutated original")
	}
}

func int64Pointer(value int64) *int64 {
	return &value
}
