package memcached

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

// TestConsistentServerSelector_PickServer tests that with a replication factor of defaultNodeReplicationFactor keys
// are evenly distributed among different amounts of servers with an error of 10%
func TestConsistentServerSelector_PickServer(t *testing.T) {
	keys := make([]string, 1e6)
	for i := range keys {
		keys[i] = fmt.Sprintf("%20d", i)
	}

	servers := make([]string, 1000)
	for i := range servers {
		servers[i] = fmt.Sprintf("127.0.0.1:%d", 1000+i)
	}
	for _, serversLen := range []int{2, 5, 10, 25, 50} {
		for _, replicationFactor := range []int{defaultNodeReplicationFactor} {
			t.Run(fmt.Sprintf("%d keys on %d servers with replication=%d", len(keys), serversLen, replicationFactor), func(t *testing.T) {
				ss, err := newConsistentServerSelector(servers[:serversLen], replicationFactor)
				require.NoError(t, err)

				keysOnServer := map[string]int{}
				for _, k := range keys {
					addr, err := ss.PickServer(k)
					require.NoError(t, err)
					keysOnServer[addr.String()]++
				}

				expectedKeysPerServer := float64(len(keys)) / float64(serversLen)
				for server, keys := range keysOnServer {
					assert.InEpsilon(t, expectedKeysPerServer, keys, 0.1, "Server %s has %d keys, but expected is %f keys per server", server, keys, expectedKeysPerServer)
				}
			})
		}
	}
}

// BenchmarkConsistentServerSelector_PickServer
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_2_servers_with_replication=10
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_2_servers_with_replication=10-12         	 3386896	       333 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_2_servers_with_replication=100
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_2_servers_with_replication=100-12        	 3160695	       381 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_2_servers_with_replication=1000
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_2_servers_with_replication=1000-12       	 2610499	       458 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_2_servers_with_replication=10000
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_2_servers_with_replication=10000-12      	 2076230	       581 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_5_servers_with_replication=10
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_5_servers_with_replication=10-12         	 3354843	       351 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_5_servers_with_replication=100
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_5_servers_with_replication=100-12        	 2796002	       427 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_5_servers_with_replication=1000
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_5_servers_with_replication=1000-12       	 2264095	       532 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_5_servers_with_replication=10000
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_5_servers_with_replication=10000-12      	 1773258	       674 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_10_servers_with_replication=10
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_10_servers_with_replication=10-12        	 3168775	       378 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_10_servers_with_replication=100
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_10_servers_with_replication=100-12       	 2597354	       469 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_10_servers_with_replication=1000
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_10_servers_with_replication=1000-12      	 2078637	       584 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_10_servers_with_replication=10000
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_10_servers_with_replication=10000-12     	 1333578	       892 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_50_servers_with_replication=10
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_50_servers_with_replication=10-12        	 2720323	       445 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_50_servers_with_replication=100
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_50_servers_with_replication=100-12       	 2144714	       556 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_50_servers_with_replication=1000
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_50_servers_with_replication=1000-12      	 1667799	       712 ns/op
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_50_servers_with_replication=10000
// BenchmarkConsistentServerSelector_PickServer/1000000_keys_on_50_servers_with_replication=10000-12     	  873220	      1363 ns/op
func BenchmarkConsistentServerSelector_PickServer(b *testing.B) {
	keys := make([]string, 1e6)
	for i := range keys {
		keys[i] = fmt.Sprintf("%20d", i)
	}

	servers := make([]string, 1000)
	for i := range servers {
		servers[i] = fmt.Sprintf("127.0.0.1:%d", 1000+i)
	}
	for _, serversLen := range []int{2, 5, 10, 50} {
		for _, replicationFactor := range []int{10, 100, 1000, 10000} {
			b.Run(fmt.Sprintf("%d keys on %d servers with replication=%d", len(keys), serversLen, replicationFactor), func(b *testing.B) {
				ss, _ := newConsistentServerSelector(servers[:serversLen], replicationFactor)
				b.ResetTimer()

				keysOnServer := map[string]int{}
				for i := 0; i < b.N; i++ {
					key := keys[i%len(keys)]
					addr, _ := ss.PickServer(key)
					keysOnServer[addr.String()]++
				}
			})
		}
	}
}
