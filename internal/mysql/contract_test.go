package mysql_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Taier05/InfraView/internal/mysql"
	"github.com/Taier05/InfraView/internal/mysql/mysqltest"
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

func TestRunContractValidatesSnapshotFixture(t *testing.T) {
	mysqltest.RunContract(t, contractFixtureProvider{})
}

type contractFixtureProvider struct{}

func (contractFixtureProvider) MySQLSnapshot(context.Context) (mysql.Snapshot, error) {
	return mysql.Snapshot{Instances: []mysql.Instance{{
		ID:      mysql.StableInstanceID("contract-host", "contract-mysql", "203.0.113.10:3306"),
		Name:    "contract-mysql",
		Address: "203.0.113.10:3306",
		Host:    "contract-host",
	}}}, nil
}
