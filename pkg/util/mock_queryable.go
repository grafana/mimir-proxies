// Copied and adapted from backend-enterprise

package util

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util/annotations"

	remotereadstorage "github.com/grafana/mimir-proxies/pkg/remoteread/storage"
)

// MockQueryable can be used as a querier or queryable in test cases, it is
// only intended for unit tests and not for any production use.
// It takes a series of expected querier calls and select calls on the querier,
// it validates whether all the expected calls have been received and none more.
// For each expected call the return value can also be defined.
type MockQueryable struct {
	mu sync.Mutex
	TB testing.TB

	// List of expected calls to the Queryable's .Querier() method,
	// order is not enforced to allow for multi-threaded use.
	ExpectedQuerierCalls []Call

	// List of expected calls to the Querier's .Select() method,
	// order is not enforced to allow for multi-threaded use.
	ExpectedSelectCalls []SelectCall

	// List of expected calls to the Querier's .LabelValues() method,
	// order is not enforced to allow for multi-threaded use.
	ExpectedLabelValuesCalls []LabelValuesCall

	// List of expected calls to the Querier's .Series() method,
	// order is not enforced to allow for multi-threaded use.
	ExpectedSeriesCalls []SeriesCall

	ExpectedLabelNamesCalls []LabelNamesCall

	UnlimitedCalls bool
}

func (m *MockQueryable) Close() error {
	return nil
}

type AnyType struct{}

var Any = &AnyType{}

type Call struct {
	ExpectedMinT interface{}
	ExpectedMaxT interface{}
	ReturnErr    error
}

type SelectCall struct {
	ArgSortSeries bool
	ArgHints      *storage.SelectHints
	ArgMatchers   []*labels.Matcher
	ReturnValue   func() storage.SeriesSet
}

type LabelValuesCall struct {
	Label        string
	Matchers     []*labels.Matcher
	ReturnValues []string
	ReturnErr    error
}

type SeriesCall struct {
	Matchers     []string
	ReturnValues []map[string]string
}

type LabelNamesCall struct {
	ReturnValues   []string
	ReturnWarnings annotations.Annotations
	ReturnErr      error
}

func (m *MockQueryable) Querier(mint, maxt int64) (remotereadstorage.Querier, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for callIdx := range m.ExpectedQuerierCalls {
		var expectedMinT int64
		if m.ExpectedQuerierCalls[callIdx].ExpectedMinT == Any {
			expectedMinT = mint
		} else {
			switch v := m.ExpectedQuerierCalls[callIdx].ExpectedMinT.(type) {
			case int:
				expectedMinT = int64(v)
			case int64:
				expectedMinT = v
			}
		}

		var expectedMaxT int64
		if m.ExpectedQuerierCalls[callIdx].ExpectedMaxT == Any {
			expectedMaxT = maxt
		} else {
			switch v := m.ExpectedQuerierCalls[callIdx].ExpectedMaxT.(type) {
			case int:
				expectedMaxT = int64(v)
			case int64:
				expectedMaxT = v
			}
		}

		if expectedMinT != mint || expectedMaxT != maxt {
			continue
		}

		returnErr := m.ExpectedQuerierCalls[callIdx].ReturnErr

		if !m.UnlimitedCalls {
			m.ExpectedQuerierCalls = append(m.ExpectedQuerierCalls[:callIdx], m.ExpectedQuerierCalls[callIdx+1:]...)
		}

		// mockQueryable satisfies both interfaces
		// storage.Queryable and storage.Querier.
		return m, returnErr
	}

	m.TB.Fatalf("MockQueryable: Unexpected call to .Querier(ctx, %d, %d), not found in remaining expected calls %+v", mint, maxt, m.ExpectedQuerierCalls)
	return nil, errors.New("no querier")
}

func (m *MockQueryable) Select(_ context.Context,
	sortSeries bool,
	hints *storage.SelectHints,
	matchers ...*labels.Matcher) storage.SeriesSet {
	m.mu.Lock()
	defer m.mu.Unlock()

CALLS:
	for callIdx, expectedCall := range m.ExpectedSelectCalls {
		// Compare all relevant properties of the expected calls to the given
		// arguments to decide whether this call matches an expected call.

		if expectedCall.ArgSortSeries != sortSeries {
			continue
		}
		if (expectedCall.ArgHints == nil) != (hints == nil) {
			continue
		}
		if expectedCall.ArgHints != nil &&
			(expectedCall.ArgHints.Start != hints.Start || expectedCall.ArgHints.End != hints.End) {
			continue
		}
		if len(expectedCall.ArgMatchers) != len(matchers) {
			continue
		}

		for i := range expectedCall.ArgMatchers {
			if expectedCall.ArgMatchers[i].Type != matchers[i].Type {
				continue CALLS
			}
			if expectedCall.ArgMatchers[i].Name != matchers[i].Name {
				continue CALLS
			}
			if expectedCall.ArgMatchers[i].Value != matchers[i].Value {
				continue CALLS
			}
		}

		res := m.ExpectedSelectCalls[callIdx].ReturnValue()

		if !m.UnlimitedCalls {
			m.ExpectedSelectCalls = append(m.ExpectedSelectCalls[:callIdx], m.ExpectedSelectCalls[callIdx+1:]...)
		}

		return res
	}

	m.TB.Fatalf("MockQuerier: Unexpected call to .Select(%t, %+v, %+v)", sortSeries, hints, matchers)
	return nil
}

func (m *MockQueryable) LabelValues(_ context.Context, labelName string, _ *storage.LabelHints, matchers ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for callIdx, expectedCall := range m.ExpectedLabelValuesCalls {
		if expectedCall.Label != labelName {
			continue
		}

		if !reflect.DeepEqual(expectedCall.Matchers, matchers) {
			continue
		}

		res := expectedCall.ReturnValues
		returnError := expectedCall.ReturnErr
		if !m.UnlimitedCalls {
			m.ExpectedLabelValuesCalls = append(m.ExpectedLabelValuesCalls[:callIdx],
				m.ExpectedLabelValuesCalls[callIdx+1:]...)
		}

		return res, nil, returnError
	}

	m.TB.Fatalf("MockQuerier: Unexpected call to .LabelValues(%s)", labelName)
	return nil, nil, nil
}

func (m *MockQueryable) LabelNames(ctx context.Context, _ *storage.LabelHints, matchers ...*labels.Matcher) ([]string, annotations.Annotations, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.ExpectedLabelNamesCalls) > 0 {
		returnValues := m.ExpectedLabelNamesCalls[0].ReturnValues
		returnWarnings := m.ExpectedLabelNamesCalls[0].ReturnWarnings
		returnErr := m.ExpectedLabelNamesCalls[0].ReturnErr

		if !m.UnlimitedCalls {
			m.ExpectedLabelNamesCalls = m.ExpectedLabelNamesCalls[:0]
		}

		return returnValues, returnWarnings, returnErr
	}

	m.TB.Fatalf("MockQueryable: Unexpected call to .LabelNames()")
	return nil, nil, nil
}

// ValidateAllCalls checks if all the expected calls have been made, if not it
// raises a fatal error.
func (m *MockQueryable) ValidateAllCalls() {
	m.TB.Helper()

	if len(m.ExpectedQuerierCalls) > 0 {
		m.TB.Fatalf("Expected querier calls have not been made: %+v", m.ExpectedQuerierCalls)
	}

	if len(m.ExpectedSelectCalls) > 0 {
		m.TB.Fatalf("Expected select calls: %+v", m.ExpectedSelectCalls)
	}

	if len(m.ExpectedLabelValuesCalls) > 0 {
		m.TB.Fatalf("Expected label values calls: %+v", m.ExpectedLabelValuesCalls)
	}

	if len(m.ExpectedLabelNamesCalls) > 0 {
		m.TB.Fatalf("Expected label values calls: %+v", m.ExpectedLabelNamesCalls)
	}
}

func (m *MockQueryable) Series(_ context.Context, labelMatchers []string) ([]map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, expected := range m.ExpectedSeriesCalls {
		if !reflect.DeepEqual(expected.Matchers, labelMatchers) {
			continue
		}

		res := expected.ReturnValues

		if !m.UnlimitedCalls {
			m.ExpectedSeriesCalls = append(m.ExpectedSeriesCalls[:i], m.ExpectedSeriesCalls[i+1:]...)
		}

		return res, nil
	}

	return nil, nil
}
