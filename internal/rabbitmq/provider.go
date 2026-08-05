package rabbitmq

import (
	"context"
	"errors"
)

var ErrUnavailable = errors.New("rabbitmq data source: unavailable")

type Provider interface {
	RabbitMQSnapshot(context.Context) (Snapshot, error)
}
