package mysql_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Taier05/InfraView/internal/mysql"
)

func TestStableInstanceIDUsesAllIdentityLabels(t *testing.T) {
	first := mysql.StableInstanceID("fixture-host-a", "fixture-mysql", "192.0.2.10:3306")
	again := mysql.StableInstanceID("fixture-host-a", "fixture-mysql", "192.0.2.10:3306")
	changedAddress := mysql.StableInstanceID("fixture-host-a", "fixture-mysql", "192.0.2.11:3306")
	if first == "" || first != again || first == changedAddress {
		t.Fatalf("stable IDs do not preserve the identity contract")
	}
	if strings.Contains(first, "fixture") || strings.Contains(first, "192.0.2.") {
		t.Fatalf("stable ID exposes raw labels: %q", first)
	}
}

func RunContract(t *testing.T, provider mysql.Provider) {
	t.Helper()
	snapshot, err := provider.MySQLSnapshot(context.Background())
	if err != nil {
		t.Fatalf("MySQLSnapshot() error = %v", err)
	}
	seen := map[string]struct{}{}
	for _, instance := range snapshot.Instances {
		if instance.ID == "" || instance.Name == "" || instance.Address == "" || instance.Host == "" {
			t.Fatalf("incomplete fixture instance")
		}
		if _, exists := seen[instance.ID]; exists {
			t.Fatalf("duplicate stable ID")
		}
		seen[instance.ID] = struct{}{}
	}
}
