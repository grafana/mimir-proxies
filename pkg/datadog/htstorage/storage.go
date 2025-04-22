package htstorage

import (
	"context"
	"time"

	"github.com/prometheus/prometheus/prompb"
)

// Storage specifies necessary methods to storage and fetch datadog host labels that are periodically sent by the agents
//
//go:generate mockery --case underscore --output htstoragemock --outpkg htstoragemock --name Storage
//go:generate mockery --case underscore --inpackage --testonly --name Storage
type Storage interface {
	Getter
	Set(ctx context.Context, hostName string, labels []prompb.Label) error
}

//go:generate mockery --case underscore --output htstoragemock --outpkg htstoragemock --name Getter
//go:generate mockery --case underscore --inpackage --testonly --name Getter
type Getter interface {
	Get(ctx context.Context, hostName string) ([]prompb.Label, error)
	GetAll(ctx context.Context, from time.Time) (map[string]Host, error)
}

type Host struct {
	Labels []prompb.Label
	// The last time the host reported their labels. The accuracy of the timestamp depends on the Getter implementation
	LastReportedTime time.Time
}

type NotFoundError struct {
	msg string
	err error
}

func (e NotFoundError) Error() string {
	return e.msg
}

func (e NotFoundError) Message() string {
	return e.msg
}

func (e NotFoundError) Unwrap() error {
	return e.err
}
