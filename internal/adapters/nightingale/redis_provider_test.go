package nightingale

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Taier05/InfraView/internal/redis"
)

func TestRedisPromQLIsFixedAndReturnsACopy(t *testing.T) {
	want := []string{
		"redis_up",
		"redis_uptime_in_seconds",
		"redis_cluster_enabled",
		"redis_used_memory",
		"redis_maxmemory",
		"redis_connected_clients",
		"redis_maxclients",
		"redis_blocked_clients",
		"redis_instantaneous_ops_per_sec",
		"redis_keyspace_hitrate",
		"sum by (ident, instance, address, replica_role) (redis_keyspace_keys)",
		"rate(redis_expired_keys[5m])",
		"rate(redis_evicted_keys[5m])",
		"rate(redis_rejected_connections[5m])",
		"redis_connected_slaves",
		"redis_master_link_status",
		"redis_master_last_io_seconds_ago",
		"redis_master_sync_in_progress",
		"max by (ident, instance, address, replica_role) (redis_replication_lag)",
		"tlast_over_time(redis_up[24h])",
		"tlast_over_time(redis_uptime_in_seconds[24h])",
	}
	got := redisPromQL()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("redisPromQL() = %#v", got)
	}
	got[0] = "changed"
	if redisPromQL()[0] != "redis_up" {
		t.Fatal("global query list was mutated")
	}
}

func TestRedisSnapshotUsesOneFixedBatchAndMapsSafeFields(t *testing.T) {
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assertAuthenticatedJSONRequest(t, request)
		paths = append(paths, request.URL.Path)
		switch request.URL.Path {
		case "/api/n9e/datasource/brief":
			writeFixture(t, w, "datasource-brief.json")
		case "/api/n9e/query-instant-batch":
			assertRedisBatchRequest(t, request)
			fixture, err := os.ReadFile("testdata/redis-instant-batch.json")
			if err != nil {
				t.Fatal(err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(fixture)
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: fixtureToken, HTTPClient: server.Client(), Clock: fixedClock})
	snapshot, err := provider.RedisSnapshot(context.Background())
	if err != nil {
		t.Fatalf("RedisSnapshot() error = %v", err)
	}
	if !reflect.DeepEqual(paths, []string{"/api/n9e/datasource/brief", "/api/n9e/query-instant-batch"}) {
		t.Fatalf("request paths = %#v", paths)
	}
	if len(snapshot.Instances) != 1 {
		t.Fatalf("instances = %#v", snapshot.Instances)
	}
	instance := snapshot.Instances[0]
	if instance.ID == "" || instance.Address != "192.0.2.40:6379" || instance.Availability != redis.AvailabilityUp || instance.Role != redis.RoleSlave {
		t.Fatalf("identity/role = %#v", instance)
	}
	if instance.ReportedAt.Unix() != 1785200000 || !instance.CollectionTracked {
		t.Fatalf("freshness = %#v", instance)
	}
	if instance.UsedMemoryBytes == nil || *instance.UsedMemoryBytes != 64 || instance.Keys == nil || *instance.Keys != 7 {
		t.Fatalf("integer fields = %#v", instance)
	}
	if instance.HitRate == nil || *instance.HitRate != 0.75 || instance.Replication.MasterLinkUp == nil || !*instance.Replication.MasterLinkUp {
		t.Fatalf("ratio/replication = %#v", instance)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"fixture-ident", "fixture-instance", "replica_ip", "replica_port", "replica_id", "redis_up"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("snapshot exposes %q", forbidden)
		}
	}
}

func TestBuildRedisSnapshotRejectsInvalidInventoryAndKeepsInvalidOptionalValuesMissing(t *testing.T) {
	groups := redisInstantGroups()
	groups[redisUsedMemoryQuery] = []instantSeries{
		redisInstantSeries(redisFixtureLabels("redis_used_memory", "slave"), 1785200000, "64"),
		redisInstantSeries(redisFixtureLabels("redis_used_memory", "slave"), 1785200000, "65"),
	}
	groups[redisHitRateQuery] = []instantSeries{redisInstantSeries(redisFixtureLabels("redis_keyspace_hitrate", "slave"), 1785200000, "1.5")}
	groups[redisMasterLinkStatusQuery] = []instantSeries{redisInstantSeries(redisFixtureLabels("redis_master_link_status", "slave"), 1785200000, "2")}

	snapshot, err := buildRedisSnapshot(groups)
	if err != nil {
		t.Fatalf("buildRedisSnapshot() error = %v", err)
	}
	instance := snapshot.Instances[0]
	if instance.UsedMemoryBytes != nil || instance.HitRate != nil || instance.Replication.MasterLinkUp != nil {
		t.Fatalf("invalid or conflicting values were retained: %#v", instance)
	}

	groups = redisInstantGroups()
	groups[redisInventoryQuery] = append(groups[redisInventoryQuery], groups[redisInventoryQuery][0])
	if _, err := buildRedisSnapshot(groups); !errors.Is(err, redis.ErrUnavailable) {
		t.Fatalf("duplicate inventory error = %v", err)
	}
}

func TestRedisSnapshotWrapsProtocolFailuresWithoutSecrets(t *testing.T) {
	const secret = "fixture-redis-secret"
	tests := []struct {
		name   string
		status int
		ctype  string
		body   string
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, ctype: "application/json", body: `{"dat":null,"err":"` + secret + `"}`},
		{name: "non json", status: http.StatusBadGateway, ctype: "text/html", body: `<html>` + secret + `</html>`},
		{name: "null data", status: http.StatusOK, ctype: "application/json", body: `{"dat":null,"err":""}`},
		{name: "envelope error", status: http.StatusOK, ctype: "application/json", body: `{"dat":[],"err":"` + secret + `"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/api/n9e/datasource/brief" {
					writeFixture(t, w, "datasource-brief.json")
					return
				}
				w.Header().Set("Content-Type", test.ctype)
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()
			provider := New(Options{BaseURL: server.URL, AllowInsecureHTTP: true, Token: secret, HTTPClient: server.Client(), Clock: fixedClock})
			_, err := provider.RedisSnapshot(context.Background())
			if !errors.Is(err, redis.ErrUnavailable) {
				t.Fatalf("error = %v", err)
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), server.URL) {
				t.Fatalf("error exposes sensitive input: %v", err)
			}
		})
	}
}

func assertRedisBatchRequest(t *testing.T, request *http.Request) {
	t.Helper()
	var body batchRequest
	decodeRequest(t, request, &body)
	want := redisPromQL()
	if request.Method != http.MethodPost || body.DatasourceID != 7 || len(body.Queries) != len(want) {
		t.Fatalf("Redis batch shape = %#v", body)
	}
	for index, query := range body.Queries {
		if query.Query != want[index] || query.Time != fixedClock().Unix() {
			t.Fatalf("Redis query %d = %#v", index, query)
		}
	}
}

func redisFixtureLabels(metric, role string) map[string]string {
	labels := map[string]string{"__name__": metric, "ident": "fixture-ident-a", "instance": "fixture-instance-a", "address": "192.0.2.40:6379"}
	if role != "" {
		labels["replica_role"] = role
	}
	return labels
}

func redisInstantSeries(labels map[string]string, timestamp int64, value string) instantSeries {
	return instantSeries{Metric: labels, Value: rawInstantValue(timestamp, value)}
}

func redisInstantGroups() [][]instantSeries {
	values := []string{"1", "3600", "1", "64", "128", "12", "100", "0", "25", "0.75", "7", "0.2", "0", "0", "0", "1", "1", "0", "2"}
	groups := make([][]instantSeries, redisQueryCount)
	for index, value := range values {
		role := "slave"
		if index == redisUpQuery {
			role = ""
		}
		groups[index] = []instantSeries{redisInstantSeries(redisFixtureLabels(redisPromQL()[index], role), 1785200100, value)}
	}
	groups[redisInventoryQuery] = []instantSeries{redisInstantSeries(redisFixtureLabels("redis_up", ""), 1785200100, "1785200000")}
	groups[redisHistoricalRoleQuery] = []instantSeries{redisInstantSeries(redisFixtureLabels("redis_uptime_in_seconds", "slave"), 1785200100, "1785200000")}
	return groups
}
