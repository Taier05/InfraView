package nightingale

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/datasource"
)

type envelope struct {
	Data json.RawMessage `json:"dat"`
	Err  json.RawMessage `json:"err"`
}

type targetPage struct {
	List  *[]targetRecord `json:"list"`
	Total *int            `json:"total"`
}

type targetRecord struct {
	Ident    string `json:"ident"`
	HostIP   string `json:"host_ip"`
	OS       string `json:"os"`
	BeatTime int64  `json:"beat_time"`
	TargetUp int    `json:"target_up"`
	CPUNum   int    `json:"cpu_num"`
}

type datasourceRecord struct {
	ID         int64  `json:"id"`
	PluginType string `json:"plugin_type"`
	IsDefault  bool   `json:"is_default"`
}

type datasourceDiscovery struct {
	done chan struct{}
	id   int64
	err  error
}

type instantRequest struct {
	DatasourceID int64          `json:"datasource_id"`
	Queries      []instantQuery `json:"queries"`
}

type instantQuery struct {
	Time  int64  `json:"time"`
	Query string `json:"query"`
}

type rangeRequest struct {
	DatasourceID int64        `json:"datasource_id"`
	Queries      []rangeQuery `json:"queries"`
}

type rangeQuery struct {
	Start int64  `json:"start"`
	End   int64  `json:"end"`
	Step  int64  `json:"step"`
	Query string `json:"query"`
}

type instantSeries struct {
	Metric map[string]string `json:"metric"`
	Value  []json.RawMessage `json:"value"`
}

type rangeSeries struct {
	Metric map[string]string   `json:"metric"`
	Values [][]json.RawMessage `json:"values"`
}

const (
	jsonUnixMinSeconds          int64 = -62167219200
	jsonUnixMaxSecondsExclusive int64 = 253402300800
)

func (p *Provider) get(ctx context.Context, path string, query url.Values, target any) error {
	return p.do(ctx, http.MethodGet, path, query, nil, target)
}

func (p *Provider) post(ctx context.Context, path string, body any, target any) error {
	return p.do(ctx, http.MethodPost, path, nil, body, target)
}

func (p *Provider) do(ctx context.Context, method, path string, query url.Values, body any, target any) error {
	requestURL := *p.baseURL
	requestURL.Path = strings.TrimRight(requestURL.Path, "/") + path
	requestURL.RawQuery = query.Encode()

	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return unavailableError()
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), requestBody)
	if err != nil {
		return unavailableError()
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-User-Token", p.token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := p.httpClient.Do(request)
	if err != nil {
		return unavailableError()
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !isJSONContentType(response.Header.Get("Content-Type")) {
		return unavailableError()
	}

	limited := io.LimitReader(response.Body, p.maxResponseBytes+1)
	encoded, err := io.ReadAll(limited)
	if err != nil || int64(len(encoded)) > p.maxResponseBytes {
		return unavailableError()
	}
	var result envelope
	if err := json.Unmarshal(encoded, &result); err != nil || len(result.Data) == 0 || bytes.Equal(bytes.TrimSpace(result.Data), []byte("null")) || len(result.Err) == 0 || envelopeHasError(result.Err) {
		return unavailableError()
	}
	if err := json.Unmarshal(result.Data, target); err != nil {
		return unavailableError()
	}
	return nil
}

func isJSONContentType(raw string) bool {
	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return false
	}
	return mediaType == "application/json"
}

func envelopeHasError(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var message string
	if err := json.Unmarshal(raw, &message); err != nil {
		return true
	}
	return message != ""
}

func (p *Provider) discoverDatasourceID(ctx context.Context) (int64, error) {
	p.datasourceMu.Lock()
	if p.datasourceID > 0 {
		id := p.datasourceID
		p.datasourceMu.Unlock()
		return id, nil
	}
	if flight := p.datasourceFly; flight != nil {
		done := flight.done
		p.datasourceMu.Unlock()
		select {
		case <-done:
			if flight.err != nil {
				return 0, flight.err
			}
			if flight.id <= 0 {
				return 0, unavailableError()
			}
			return flight.id, nil
		case <-ctx.Done():
			return 0, unavailableError()
		}
	}
	flight := &datasourceDiscovery{done: make(chan struct{})}
	p.datasourceFly = flight
	p.datasourceMu.Unlock()

	id, err := p.loadDatasourceID(ctx)
	p.datasourceMu.Lock()
	if err == nil && id > 0 {
		p.datasourceID = id
		flight.id = id
	} else {
		if err == nil {
			err = unavailableError()
		}
		flight.err = err
	}
	p.datasourceFly = nil
	close(flight.done)
	p.datasourceMu.Unlock()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (p *Provider) loadDatasourceID(ctx context.Context) (int64, error) {
	var records []datasourceRecord
	if err := p.get(ctx, "/api/n9e/datasource/brief", nil, &records); err != nil {
		return 0, err
	}
	candidates := make([]datasourceRecord, 0, len(records))
	for _, record := range records {
		if record.PluginType == "prometheus" && record.ID > 0 {
			candidates = append(candidates, record)
		}
	}
	if len(candidates) == 0 {
		return 0, unavailableError()
	}
	sortDatasourceRecords(candidates)
	return candidates[0].ID, nil
}

func (p *Provider) queryInstant(ctx context.Context, queries []string) ([][]instantSeries, error) {
	datasourceID, err := p.discoverDatasourceID(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]instantQuery, len(queries))
	now := p.now().Unix()
	for i, query := range queries {
		items[i] = instantQuery{Time: now, Query: query}
	}
	var result [][]instantSeries
	if err := p.post(ctx, "/api/n9e/query-instant-batch", instantRequest{DatasourceID: datasourceID, Queries: items}, &result); err != nil {
		return nil, err
	}
	if len(result) != len(queries) {
		return nil, unavailableError()
	}
	return result, nil
}

func (p *Provider) queryRange(ctx context.Context, queries []rangeQuery) ([][]rangeSeries, error) {
	datasourceID, err := p.discoverDatasourceID(ctx)
	if err != nil {
		return nil, err
	}
	var result [][]rangeSeries
	if err := p.post(ctx, "/api/n9e/query-range-batch", rangeRequest{DatasourceID: datasourceID, Queries: queries}, &result); err != nil {
		return nil, err
	}
	if len(result) != len(queries) {
		return nil, unavailableError()
	}
	return result, nil
}

func parseInstantValue(raw []json.RawMessage) (float64, time.Time, bool) {
	if len(raw) != 2 {
		return 0, time.Time{}, false
	}
	timestamp, ok := parseUnixTime(raw[0])
	if !ok {
		return 0, time.Time{}, false
	}
	value, ok := parseFiniteFloat(raw[1])
	if !ok {
		return 0, time.Time{}, false
	}
	return value, timestamp, true
}

func parseRangePoints(values [][]json.RawMessage) []datasource.Point {
	points := make([]datasource.Point, 0, len(values))
	for _, raw := range values {
		if len(raw) != 2 {
			continue
		}
		timestamp, ok := parseUnixTime(raw[0])
		if !ok {
			continue
		}
		value, valid := parseFiniteFloat(raw[1])
		var pointer *float64
		if valid {
			copy := value
			pointer = &copy
		}
		points = append(points, datasource.Point{Timestamp: timestamp, Value: pointer})
	}
	return points
}

func parseUnixTime(raw json.RawMessage) (time.Time, bool) {
	value, err := strconv.ParseFloat(strings.Trim(string(raw), `"`), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return time.Time{}, false
	}
	seconds, fraction := math.Modf(value)
	unixSeconds, ok := finiteInt64(seconds)
	if !ok {
		return time.Time{}, false
	}
	nanoseconds, ok := finiteInt64(fraction * float64(time.Second))
	if !ok {
		return time.Time{}, false
	}
	return jsonUnixTime(unixSeconds, nanoseconds)
}

func jsonUnixTime(seconds, nanoseconds int64) (time.Time, bool) {
	if seconds < jsonUnixMinSeconds || seconds >= jsonUnixMaxSecondsExclusive || (seconds == jsonUnixMinSeconds && nanoseconds < 0) {
		return time.Time{}, false
	}
	return time.Unix(seconds, nanoseconds).UTC(), true
}

func parseFiniteFloat(raw json.RawMessage) (float64, bool) {
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		text = string(raw)
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}
