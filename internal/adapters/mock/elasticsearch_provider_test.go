package mock_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/Taier05/InfraView/internal/adapters/mock"
	"github.com/Taier05/InfraView/internal/elasticsearch"
	"github.com/Taier05/InfraView/internal/elasticsearch/elasticsearchtest"
)

func TestElasticsearchProviderContract(t *testing.T) {
	elasticsearchtest.RunContract(t, mock.NewElasticsearch())
}

func TestElasticsearchProviderContainsDeterministicScenarios(t *testing.T) {
	first, err := mock.NewElasticsearch().ElasticsearchSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := mock.NewElasticsearch().ElasticsearchSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("mock snapshot is not deterministic")
	}
	if len(first.Clusters) < 3 || len(first.Nodes) < 3 {
		t.Fatal("mock must contain multiple clusters and nodes")
	}

	healths := map[elasticsearch.Health]bool{}
	var multiRoleDataNode, dedicatedMaster, missingOptionalValues bool
	var resourceWarning, resourceCritical, rejectionWarning bool
	for _, cluster := range first.Clusters {
		healths[cluster.Health] = true
	}
	for _, node := range first.Nodes {
		if node.DataNode && len(node.Roles) > 1 {
			multiRoleDataNode = true
		}
		if !node.DataNode && len(node.Roles) == 1 && node.Roles[0] == elasticsearch.RoleMaster {
			dedicatedMaster = true
		}
		if node.IndexRate == nil || node.SearchRate == nil || node.RejectedRate == nil {
			missingOptionalValues = true
		}
		if node.HeapUsedBytes != nil && node.HeapMaxBytes != nil && *node.HeapMaxBytes > 0 {
			heapPercent := float64(*node.HeapUsedBytes) / float64(*node.HeapMaxBytes) * 100
			if heapPercent >= 75 && heapPercent < 85 {
				resourceWarning = true
			}
			if heapPercent >= 85 {
				resourceCritical = true
			}
		}
		if node.DiskUsagePercent != nil {
			if *node.DiskUsagePercent >= 85 && *node.DiskUsagePercent < 90 {
				resourceWarning = true
			}
			if *node.DiskUsagePercent >= 90 {
				resourceCritical = true
			}
		}
		if node.RejectedRate != nil && *node.RejectedRate > 0 {
			rejectionWarning = true
		}
	}
	if !healths[elasticsearch.HealthGreen] || !healths[elasticsearch.HealthYellow] || !healths[elasticsearch.HealthRed] {
		t.Fatal("mock must cover green, yellow and red cluster health")
	}
	if !multiRoleDataNode || !dedicatedMaster || !missingOptionalValues {
		t.Fatal("mock must cover role and missing optional-value scenarios")
	}
	if !resourceWarning || !resourceCritical || !rejectionWarning {
		t.Fatal("mock must cover resource warning, resource critical and rejection warning scenarios")
	}
}
