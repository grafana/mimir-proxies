package convert

import (
	"sync"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func TestProgress(t *testing.T) {
	p := NewProgress(log.NewNopLogger())

	count := 1000
	threads := 1000

	wg := sync.WaitGroup{}
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			for j := 0; j < count; j++ {
				p.IncProcessed()
				p.IncSkipped()
			}
			wg.Done()
		}()
	}

	wg.Wait()
	require.Equal(t, uint64(1000000), p.GetProcessedCount())
	require.Equal(t, uint64(1000000), p.GetSkippedCount())
}
