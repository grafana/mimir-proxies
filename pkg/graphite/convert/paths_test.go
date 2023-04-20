package convert

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPathsForWorker(t *testing.T) {
	tests := []struct {
		name        string
		paths       []string
		workerCount int
		workerID    int
		wantPaths   []string
	}{
		{
			name:        "nil",
			paths:       nil,
			workerCount: 100,
			workerID:    2,
			wantPaths:   []string{},
		},
		{
			name:        "empty",
			paths:       []string{},
			workerCount: 100,
			workerID:    2,
			wantPaths:   []string{},
		},
		{
			name:        "worker ID too big",
			paths:       []string{"foo/bar"},
			workerCount: 1,
			workerID:    20,
			wantPaths:   []string{},
		},
		{
			name: "usual behavior, first worker",
			paths: []string{
				"foo/bar1",
				"foo/bar2",
				"foo/bar3",
				"foo/bar/baz1",
				"foo/bar/baz2",
				"foo/bar/awoo1",
				"foo/bar/gibblet2",
				"foo/bar/gibblet1",
			},
			workerCount: 3,
			workerID:    0,
			wantPaths: []string{
				"foo/bar1",
				"foo/bar/baz1",
				"foo/bar/gibblet2",
			},
		},
		{
			name: "usual behavior, last worker",
			paths: []string{
				"foo/bar1",
				"foo/bar2",
				"foo/bar3",
				"foo/bar/baz1",
				"foo/bar/baz2",
				"foo/bar/awoo1",
				"foo/bar/gibblet2",
				"foo/bar/gibblet1",
			},
			workerCount: 3,
			workerID:    2,
			wantPaths: []string{
				"foo/bar3",
				"foo/bar/awoo1",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := PathsForWorker(test.paths, test.workerCount, test.workerID)
			require.Equal(t, test.wantPaths, got)
		})
	}
}
