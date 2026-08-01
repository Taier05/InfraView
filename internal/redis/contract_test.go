package redis_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Taier05/InfraView/internal/redis"
)

func TestStableInstanceIDUsesCompleteIdentity(t *testing.T) {
	id := redis.StableInstanceID("fixture-ident", "fixture-instance", "192.0.2.10:6379")
	if id == "" || id != redis.StableInstanceID("fixture-ident", "fixture-instance", "192.0.2.10:6379") {
		t.Fatal("stable ID is not deterministic")
	}
	for _, changed := range [][3]string{
		{"fixture-ident-b", "fixture-instance", "192.0.2.10:6379"},
		{"fixture-ident", "fixture-instance-b", "192.0.2.10:6379"},
		{"fixture-ident", "fixture-instance", "192.0.2.11:6379"},
	} {
		if id == redis.StableInstanceID(changed[0], changed[1], changed[2]) {
			t.Fatal("stable ID omits part of the identity")
		}
	}
	for _, raw := range []string{"fixture-ident", "fixture-instance", "192.0.2.10"} {
		if strings.Contains(id, raw) {
			t.Fatal("stable ID exposes raw identity")
		}
	}
}

func TestStableInstanceIDRejectsIncompleteIdentity(t *testing.T) {
	for _, identity := range [][3]string{
		{"", "fixture-instance", "192.0.2.10:6379"},
		{"fixture-ident", "", "192.0.2.10:6379"},
		{"fixture-ident", "fixture-instance", ""},
	} {
		if got := redis.StableInstanceID(identity[0], identity[1], identity[2]); got != "" {
			t.Fatalf("StableInstanceID() = %q for incomplete identity", got)
		}
	}
}

func TestSnapshotCloneDeepCopiesPointers(t *testing.T) {
	used := int64(64)
	lag := 3.5
	link := true
	original := redis.Snapshot{Instances: []redis.Instance{{
		ID:              "fixture-id",
		Address:         "192.0.2.10:6379",
		UsedMemoryBytes: &used,
		Replication: redis.Replication{
			MasterLinkUp:           &link,
			WorstReplicaLagSeconds: &lag,
		},
	}}}

	cloned := original.Clone()
	*cloned.Instances[0].UsedMemoryBytes = 128
	*cloned.Instances[0].Replication.MasterLinkUp = false
	*cloned.Instances[0].Replication.WorstReplicaLagSeconds = 9

	if *original.Instances[0].UsedMemoryBytes != 64 || !*original.Instances[0].Replication.MasterLinkUp || *original.Instances[0].Replication.WorstReplicaLagSeconds != 3.5 {
		t.Fatal("Clone() shares pointer fields with its input")
	}
	if reflect.DeepEqual(original, cloned) {
		t.Fatal("Clone() did not preserve independent values")
	}
}

func TestRedisDomainEnumsAreCanonical(t *testing.T) {
	if got := []redis.Availability{redis.AvailabilityUp, redis.AvailabilityDown, redis.AvailabilityUnknown}; !reflect.DeepEqual(got, []redis.Availability{"up", "down", "unknown"}) {
		t.Fatalf("availability values = %v", got)
	}
	if got := []redis.Role{redis.RoleMaster, redis.RoleSlave, redis.RoleUnknown}; !reflect.DeepEqual(got, []redis.Role{"master", "slave", "unknown"}) {
		t.Fatalf("role values = %v", got)
	}
}
