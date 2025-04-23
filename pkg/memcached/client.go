package memcached

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/grafana/dskit/flagext"
)

// Client is an interface abstraction for *memcache.Client that has a mock
type Client interface {
	Get(key string) (item *memcache.Item, err error)
	Add(item *memcache.Item) error
	Set(item *memcache.Item) error
	CompareAndSwap(item *memcache.Item) error
	Delete(key string) error
}

type Config struct {
	Timeout      time.Duration       `yaml:"timeout"`
	MaxIdleConns int                 `yaml:"max_idle_conns"`
	Servers      flagext.StringSlice `yaml:"servers"`
}

func (c *Config) RegisterFlags(flags *flag.FlagSet) {
	c.RegisterFlagsWithPrefix("", flags)
}

// RegisterFlagsWithPrefix registers flags, adding the provided prefix if
// needed. If the prefix is not blank and doesn't end with '.', a '.' is
// appended to it.
//
//nolint:gomnd
func (c *Config) RegisterFlagsWithPrefix(prefix string, flags *flag.FlagSet) {
	if prefix != "" && !strings.HasSuffix(prefix, ".") {
		prefix += "."
	}
	flags.DurationVar(&c.Timeout, prefix+"memcached-timeout", 100*time.Millisecond, "Timeout for memcached operations.")
	flags.IntVar(&c.MaxIdleConns, prefix+"memcached-max-idle-conns", 5, "Max idle conns for each memcached server.")
	flags.Var(&c.Servers, prefix+"memcached-server", "Memcache server to use, can be passed several times, consistent hashing will be used to distribute the keys.")
}

//go:generate mockery --outpkg memcachedmock --output memcachedmock --case underscore --name Client
//go:generate mockery --case underscore --inpackage --testonly --name Client
var _ Client = &memcache.Client{}

// NewClient creates a new *memcache.Client using consistent hashing for server selection and configured by Config
func NewClient(cfg Config) (*memcache.Client, error) {
	if len(cfg.Servers) == 0 {
		return nil, fmt.Errorf("no memcached servers provided")
	}
	ss, err := newConsistentServerSelector(cfg.Servers, defaultNodeReplicationFactor)
	if err != nil {
		return nil, err
	}
	mc := memcache.NewFromSelector(ss)
	mc.Timeout = cfg.Timeout
	mc.MaxIdleConns = cfg.MaxIdleConns
	return mc, nil
}
