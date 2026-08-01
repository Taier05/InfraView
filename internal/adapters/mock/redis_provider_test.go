package mock_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/adapters/mock"
	"github.com/Taier05/InfraView/internal/redis/redistest"
)

func TestRedisProviderContract(t *testing.T) {
	redistest.RunContract(t, mock.NewRedis(fixedRedisClock))
}

func TestRedisProviderContainsDeterministicHealthScenarios(t *testing.T) {
	provider := mock.NewRedis(fixedRedisClock)
	first, err := provider.RedisSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := provider.RedisSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("mock snapshot is not deterministic")
	}
	if len(first.Instances) < 7 {
		t.Fatal("mock must cover normal, warning, critical and unknown scenarios")
	}

	if first.Instances[0].UsedMemoryBytes == nil {
		t.Fatal("fixture must contain pointer data")
	}
	original := *second.Instances[0].UsedMemoryBytes
	*first.Instances[0].UsedMemoryBytes = original + 1
	third, err := provider.RedisSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if third.Instances[0].UsedMemoryBytes == nil || *third.Instances[0].UsedMemoryBytes != original {
		t.Fatal("mock returned shared pointer state")
	}
}

func fixedRedisClock() time.Time {
	return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
}
