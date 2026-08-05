package mock_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/adapters/mock"
	"github.com/Taier05/InfraView/internal/javaapp/javaapptest"
)

func TestJavaProviderContract(t *testing.T) {
	javaapptest.RunContract(t, mock.NewJava(fixedJavaClock))
}

func TestJavaProviderContainsDeterministicDisplayScenarios(t *testing.T) {
	provider := mock.NewJava(fixedJavaClock)
	first, err := provider.JavaSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := provider.JavaSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("mock snapshot is not deterministic")
	}

	var complete, healthFailed, missingRequired, collectionWarning bool
	for _, service := range first.Services {
		if !strings.HasPrefix(service.Name, "fixture-") || !strings.HasPrefix(service.Address, "fixture-") {
			t.Fatal("mock exposed a non-fixture identity")
		}
		complete = complete || (service.HealthLatencyMilliseconds != nil && service.HealthUp != nil && service.PortUp != nil && service.ProcessUp != nil && service.PortConsistent != nil && service.ProcessCount != nil && service.ProcessMemoryBytes != nil && service.ProcessCPUPercent != nil && service.ProcessMemoryPercent != nil && service.ProcessStartTimeSeconds != nil)
		healthFailed = healthFailed || service.HealthUp != nil && !*service.HealthUp
		missingRequired = missingRequired || service.PortUp == nil || service.ProcessUp == nil || service.ProcessCount == nil
		collectionWarning = collectionWarning || service.CollectionTracked && service.ReportedAt == fixedJavaClock().Add(-2*time.Minute)
	}
	if !complete || !healthFailed || !missingRequired || !collectionWarning {
		t.Fatalf("mock scenarios complete=%t healthFailed=%t missingRequired=%t collectionWarning=%t", complete, healthFailed, missingRequired, collectionWarning)
	}

	original := *second.Services[0].ProcessCount
	*first.Services[0].ProcessCount = original + 1
	third, err := provider.JavaSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if third.Services[0].ProcessCount == nil || *third.Services[0].ProcessCount != original {
		t.Fatal("mock returned shared pointer state")
	}
}

func fixedJavaClock() time.Time {
	return time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
}
