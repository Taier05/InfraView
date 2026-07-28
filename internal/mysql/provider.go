package mysql

import (
	"context"
	"errors"
)

var ErrUnavailable = errors.New("mysql data source: unavailable")

type Provider interface {
	MySQLSnapshot(context.Context) (Snapshot, error)
}
