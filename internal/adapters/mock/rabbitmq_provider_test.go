package mock_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/Taier05/InfraView/internal/adapters/mock"
	"github.com/Taier05/InfraView/internal/rabbitmq/rabbitmqtest"
)

func TestRabbitMQProviderContract(t *testing.T) {
	rabbitmqtest.RunContract(t, mock.NewRabbitMQ())
}

func TestRabbitMQProviderContainsDeterministicHealthScenarios(t *testing.T) {
	provider := mock.NewRabbitMQ()
	first, err := provider.RabbitMQSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := provider.RabbitMQSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("mock snapshot is not deterministic")
	}

	var normal, warning, critical, unknown bool
	for _, node := range first.Nodes {
		if node.CollectionTracked && node.MemoryUsedBytes != nil && node.MemoryLimitBytes != nil && *node.MemoryUsedBytes < *node.MemoryLimitBytes*80/100 && node.MemoryAlarm != nil && !*node.MemoryAlarm {
			normal = true
		}
		if node.CollectionTracked && node.MemoryUsedBytes != nil && node.MemoryLimitBytes != nil && *node.MemoryUsedBytes >= *node.MemoryLimitBytes*80/100 && *node.MemoryUsedBytes < *node.MemoryLimitBytes*90/100 {
			warning = true
		}
		if node.MemoryAlarm != nil && *node.MemoryAlarm {
			critical = true
		}
		if !node.CollectionTracked || node.MemoryUsedBytes == nil || node.MemoryLimitBytes == nil {
			unknown = true
		}
	}
	if !normal || !warning || !critical || !unknown {
		t.Fatal("mock must cover normal, warning, critical and unknown scenarios")
	}

	original := *second.Nodes[0].Connections
	*first.Nodes[0].Connections = original + 1
	third, err := provider.RabbitMQSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if third.Nodes[0].Connections == nil || *third.Nodes[0].Connections != original {
		t.Fatal("mock returned shared pointer state")
	}
}
