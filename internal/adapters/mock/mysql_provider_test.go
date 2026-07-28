package mock_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/Taier05/InfraView/internal/adapters/mock"
	"github.com/Taier05/InfraView/internal/mysql/mysqltest"
)

func TestMySQLProviderContract(t *testing.T) {
	mysqltest.RunContract(t, mock.NewMySQL())
}

func TestMySQLProviderContainsDeterministicHealthScenarios(t *testing.T) {
	first, err := mock.NewMySQL().MySQLSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := mock.NewMySQL().MySQLSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("mock snapshot is not deterministic")
	}
	if len(first.Instances) < 6 {
		t.Fatal("mock must cover normal, warning, critical and unknown scenarios")
	}
}
