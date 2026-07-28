package mysqltest

import (
	"context"
	"testing"

	"github.com/Taier05/InfraView/internal/mysql"
)

func RunContract(t testing.TB, provider mysql.Provider) {
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
