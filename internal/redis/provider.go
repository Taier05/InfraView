package redis

import (
	"context"
	"errors"
)

var ErrUnavailable = errors.New("redis data source: unavailable")

type Provider interface {
	RedisSnapshot(context.Context) (Snapshot, error)
}
