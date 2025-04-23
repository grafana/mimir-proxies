package remoteread

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/mimir-proxies/pkg/remoteread/remotereadmock"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/util/annotations"

	"github.com/prometheus/prometheus/storage"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type MeasuredStorageQueryableTestSuite struct {
	suite.Suite

	now         time.Time
	someLatency time.Duration
	timeNow     func() time.Time

	recorderMock  *MockRecorder
	queryableMock *remotereadmock.StorageQueryableInterface
	querierMock   *remotereadmock.StorageQuerierInterface

	underTest storage.Querier

	mint, maxt int64
}

func (s *MeasuredStorageQueryableTestSuite) SetupTest() {
	s.now = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	s.timeNow = func() time.Time { return s.now }
	s.someLatency = 10 * time.Millisecond
	s.mint = 120e3
	s.maxt = 180e3

	s.recorderMock = &MockRecorder{}
	s.queryableMock = &remotereadmock.StorageQueryableInterface{}
	s.querierMock = &remotereadmock.StorageQuerierInterface{}
	s.queryableMock.On("Querier", s.mint, s.maxt).Return(s.querierMock, nil)

	var err error
	s.underTest, err = NewMeasuredStorageQueryable(s.queryableMock, s.recorderMock, s.timeNow).Querier(s.mint, s.maxt)
	s.Require().NoError(err)
}

func (s *MeasuredStorageQueryableTestSuite) TearDownTest() {
	s.recorderMock.AssertExpectations(s.T())
	s.queryableMock.AssertExpectations(s.T())
}

func (s *MeasuredStorageQueryableTestSuite) TestSelect_Success() {
	matchers := []*labels.Matcher{labels.MustNewMatcher(labels.MatchEqual, "foo", "bar")}
	emptySet := storage.EmptySeriesSet()
	s.querierMock.On("Select", mock.Anything, true, (*storage.SelectHints)(nil), matchers[0]).
		Run(s.addSomeLatency()).
		Return(emptySet).Once()
	s.recorderMock.On("measure", "StorageQuerier.Select", s.someLatency, nil).Once()

	set := s.underTest.Select(context.Background(), true, nil, matchers...)

	s.NoError(set.Err())
}

func (s *MeasuredStorageQueryableTestSuite) TestSelect_Error() {
	matchers := []*labels.Matcher{labels.MustNewMatcher(labels.MatchEqual, "foo", "bar")}
	expectedErr := context.Canceled
	errSet := storage.ErrSeriesSet(expectedErr)
	s.querierMock.On("Select", mock.Anything, true, (*storage.SelectHints)(nil), matchers[0]).
		Run(s.addSomeLatency()).
		Return(errSet).Once()
	s.recorderMock.On("measure", "StorageQuerier.Select", s.someLatency, expectedErr).Once()

	set := s.underTest.Select(context.Background(), true, nil, matchers...)

	s.ErrorIs(set.Err(), expectedErr)
}

func (s *MeasuredStorageQueryableTestSuite) TestLabelNames_Success() {
	matchers := []*labels.Matcher{labels.MustNewMatcher(labels.MatchEqual, "foo", "bar")}
	mockValue := []string{"foobar"}
	mockWarnings := annotations.Annotations{}
	s.querierMock.On("LabelNames", mock.Anything, mock.Anything, matchers[0]).
		Run(s.addSomeLatency()).
		Return(mockValue, mockWarnings, nil).Once()
	s.recorderMock.On("measure", "StorageQuerier.LabelNames", s.someLatency, nil).Once()

	actualValue, actualWarnings, err := s.underTest.LabelNames(context.Background(), nil, matchers...)

	s.NoError(err)
	s.Equal(mockValue, actualValue)
	s.Equal(mockWarnings, actualWarnings)
}

func (s *MeasuredStorageQueryableTestSuite) TestLabelValues_Success() {
	const labelName = "foobar"
	matchers := []*labels.Matcher{labels.MustNewMatcher(labels.MatchEqual, "foo", "bar")}
	mockValue := []string{"foo", "bar"}
	mockWarnings := annotations.Annotations{}
	s.querierMock.On("LabelValues", mock.Anything, labelName, mock.Anything, matchers[0]).
		Run(s.addSomeLatency()).
		Return(mockValue, mockWarnings, nil).Once()
	s.recorderMock.On("measure", "StorageQuerier.LabelValues", s.someLatency, nil).Once()

	actualValue, actualWarnings, err := s.underTest.LabelValues(context.Background(), labelName, nil, matchers...)

	s.NoError(err)
	s.Equal(mockValue, actualValue)
	s.Equal(mockWarnings, actualWarnings)
}

func (s *MeasuredStorageQueryableTestSuite) addSomeLatency() func(args mock.Arguments) {
	return func(args mock.Arguments) {
		s.now = s.now.Add(s.someLatency)
	}
}

func TestMeasuredStorageQueryableTestSuite(t *testing.T) {
	suite.Run(t, new(MeasuredStorageQueryableTestSuite))
}
