package redistest

import (
	"context"
	"testing"

	"github.com/Taier05/InfraView/internal/redis"
)

func RunContract(t testing.TB, provider redis.Provider) {
	t.Helper()
	first, err := provider.RedisSnapshot(context.Background())
	if err != nil {
		t.Fatalf("RedisSnapshot() error = %v", err)
	}
	second, err := provider.RedisSnapshot(context.Background())
	if err != nil {
		t.Fatalf("second RedisSnapshot() error = %v", err)
	}
	if len(first.Instances) == 0 || len(first.Instances) != len(second.Instances) {
		t.Fatal("RedisSnapshot() instance count is empty or unstable")
	}

	seen := map[string]struct{}{}
	for index, instance := range first.Instances {
		if instance.ID == "" || instance.Address == "" {
			t.Fatalf("incomplete fixture instance at index %d", index)
		}
		if _, exists := seen[instance.ID]; exists {
			t.Fatalf("duplicate stable ID %q", instance.ID)
		}
		seen[instance.ID] = struct{}{}
		if instance.ID != second.Instances[index].ID {
			t.Fatalf("instance ID at index %d is not stable", index)
		}
		switch instance.Role {
		case redis.RoleMaster, redis.RoleSlave, redis.RoleUnknown:
		default:
			t.Fatalf("non-canonical role %q", instance.Role)
		}
	}
}
