package memcached

import (
	"fmt"
	"net"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/serialx/hashring"
)

// defaultNodeReplicationFactor guarantees that the keys are distributed evenly for up to 50 memcache servers with a deviation of less than 10% keys
const defaultNodeReplicationFactor = 1000

// consistentServerSelector implements a memcache.ServerSelector using consistent hashing algorithm implementation
// from https://github.com/serialx/hashring
// When this was developed, other libraries were taken into account:
// - https://github.com/buraksezer/consistent was too complex as it also offers bounded loads
// - https://github.com/kkdai/consistent was tested and resulted to not distribute load properly (it ignores the replication factor, see https://github.com/kkdai/consistent/issues/2)
type consistentServerSelector struct {
	addresses map[string]net.Addr
	ring      *hashring.HashRing
}

func newConsistentServerSelector(servers []string, nodeReplicationFactor int) (consistentServerSelector, error) {
	addresses := make(map[string]net.Addr)

	weights := make(map[string]int)
	for i, addr := range servers {
		// TODO: maybe support unix addrs if needed? Copy implementation from memcache.ServerList.SetServers() then
		netAddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return consistentServerSelector{}, fmt.Errorf("can't resolve memcache address %d=%q: %w", i, addr, err)
		}
		addresses[addr] = newStaticAddr(netAddr)
		weights[addr] = nodeReplicationFactor
	}
	ring := hashring.NewWithWeights(weights)

	return consistentServerSelector{
		ring:      ring,
		addresses: addresses,
	}, nil
}

var _ memcache.ServerSelector = consistentServerSelector{}

func (c consistentServerSelector) PickServer(key string) (net.Addr, error) {
	addr, ok := c.ring.GetNode(key)
	if !ok {
		return nil, fmt.Errorf("can't pick addr from ring, maybe no memcache servers")
	}
	return c.addresses[addr], nil
}

func (c consistentServerSelector) Each(f func(net.Addr) error) error {
	for _, a := range c.addresses {
		if err := f(a); err != nil {
			return err
		}
	}
	return nil
}

// staticAddr caches the Network() and String() values from any net.Addr.
// copied from memcache package
type staticAddr struct {
	ntw, str string
}

func newStaticAddr(a net.Addr) net.Addr {
	return &staticAddr{
		ntw: a.Network(),
		str: a.String(),
	}
}

func (s *staticAddr) Network() string { return s.ntw }
func (s *staticAddr) String() string  { return s.str }
