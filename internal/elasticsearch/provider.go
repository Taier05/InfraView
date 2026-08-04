package elasticsearch

import (
	"context"
	"errors"
)

var ErrUnavailable = errors.New("elasticsearch data source: unavailable")

type Provider interface {
	ElasticsearchSnapshot(context.Context) (Snapshot, error)
}
