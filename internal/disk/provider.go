package disk

import (
	"context"
	"errors"
)

var ErrUnavailable = errors.New("disk data source: unavailable")

type Provider interface {
	SMARTSnapshot(context.Context) (Snapshot, error)
}
