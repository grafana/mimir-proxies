package writeproxy

import (
	"flag"

	"github.com/grafana/mimir-proxies/pkg/remotewrite"
)

type Config struct {
	RemoteWriteConfig remotewrite.Config `yaml:"remote_write"`
}

func (c *Config) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	c.RemoteWriteConfig.RegisterFlagsWithPrefix(prefix, f)
}

// RegisterFlags adds the flags required to config this to the given FlagSet
func (c *Config) RegisterFlags(flags *flag.FlagSet) {
	c.RegisterFlagsWithPrefix("", flags)
}
