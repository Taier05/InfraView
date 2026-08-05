package javaapp

import (
	"context"
	"errors"
)

var ErrUnavailable = errors.New("java service data source: unavailable")

type Provider interface {
	JavaSnapshot(context.Context) (Snapshot, error)
}
