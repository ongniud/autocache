package autocache

import (
	"time"
)

// Options defines the configuration options for the AutoCache.
type Options struct {
	LoadBatchSize int
	LoadInterval  time.Duration
	LoadTimeout   time.Duration
	LoadBuffSize  int
	Expiration    time.Duration
	CleanInterval time.Duration
}

func NewDefaultOptions() *Options {
	return &Options{
		LoadBatchSize: 10,
		LoadInterval:  100 * time.Millisecond,
		LoadTimeout:   3 * time.Second,
		LoadBuffSize:  1000,
		Expiration:    time.Minute * 5,
		CleanInterval: time.Minute * 10,
	}
}

// Option is a function type for configuring Options.
type Option func(*Options)

// WithLoadBatchSize sets the batch size.
func WithLoadBatchSize(size int) Option {
	return func(opts *Options) {
		opts.LoadBatchSize = size
	}
}

// WithLoadInterval sets the batch interval.
func WithLoadInterval(interval time.Duration) Option {
	return func(opts *Options) {
		opts.LoadInterval = interval
	}
}

// WithLoadTimeout sets the batch timeout.
func WithLoadTimeout(timeout time.Duration) Option {
	return func(opts *Options) {
		opts.LoadTimeout = timeout
	}
}

// WithLoadBuffSize sets the task channel size.
func WithLoadBuffSize(size int) Option {
	return func(opts *Options) {
		opts.LoadBuffSize = size
	}
}

// WithExpiration ...
func WithExpiration(exp time.Duration) Option {
	return func(opts *Options) {
		opts.Expiration = exp
	}
}

// WithCleanInterval ...
func WithCleanInterval(itv time.Duration) Option {
	return func(opts *Options) {
		opts.CleanInterval = itv
	}
}
