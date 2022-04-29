package remoteread

import (
	"context"
	"errors"
	"testing"
	"time"

	apimock2 "github.com/grafana/mimir-proxies/pkg/remoteread/apimock"

	"github.com/opentracing/opentracing-go"

	"github.com/go-kit/log"
	"github.com/grafana/mimir-proxies/pkg/ctxlog"

	"github.com/prometheus/common/model"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type MeasuredAPITestSuite struct {
	suite.Suite

	now         time.Time
	someLatency time.Duration
	timeNow     func() time.Time

	recorderMock *MockRecorder
	apiMock      *apimock2.API
	underTest    API
}

func (s *MeasuredAPITestSuite) SetupTest() {
	s.now = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	s.timeNow = func() time.Time { return s.now }
	s.someLatency = 10 * time.Millisecond

	s.recorderMock = &MockRecorder{}
	s.apiMock = &apimock2.API{}

	s.underTest = NewMeasuredAPI(s.apiMock, s.recorderMock, ctxlog.NewProvider(log.NewNopLogger()), opentracing.NoopTracer{}, s.timeNow)
}

func (s *MeasuredAPITestSuite) TearDownTest() {
	s.recorderMock.AssertExpectations(s.T())
	s.apiMock.AssertExpectations(s.T())
}

func (s *MeasuredAPITestSuite) TestQueryRange_Success() {
	query := "query"
	v1Range := v1.Range{}
	mockValue := &model.Matrix{}
	mockWarnings := v1.Warnings{}
	s.apiMock.On("QueryRange", mock.Anything, query, v1Range).
		Run(s.addSomeLatency()).
		Return(mockValue, mockWarnings, nil).Once()
	s.recorderMock.On("measure", "QueryRange", s.someLatency, nil).Once()

	actualValue, actualWarnings, err := s.underTest.QueryRange(context.Background(), query, v1Range)

	s.NoError(err)
	s.Equal(mockValue, actualValue)
	s.Equal(mockWarnings, actualWarnings)
}

func (s *MeasuredAPITestSuite) TestQueryRange_Error() {
	mockError := errors.New("yikes u did bad")
	s.apiMock.On("QueryRange", mock.Anything, mock.Anything, mock.Anything).
		Run(s.addSomeLatency()).
		Return(nil, nil, mockError).Once()
	s.recorderMock.On("measure", "QueryRange", s.someLatency, mockError).Once()

	_, _, err := s.underTest.QueryRange(context.Background(), "query", v1.Range{})

	s.Error(err)
}

func (s *MeasuredAPITestSuite) TestLabelNames_Success() {
	matches := []string{}
	startTime := time.Unix(0, 0)
	endTime := time.Unix(15, 0)
	mockValue := []string{}
	mockWarnings := v1.Warnings{}
	s.apiMock.On("LabelNames", mock.Anything, matches, startTime, endTime).
		Run(s.addSomeLatency()).
		Return(mockValue, mockWarnings, nil).Once()
	s.recorderMock.On("measure", "LabelNames", s.someLatency, nil).Once()

	actualValue, actualWarnings, err := s.underTest.LabelNames(context.Background(), matches, startTime, endTime)

	s.NoError(err)
	s.Equal(mockValue, actualValue)
	s.Equal(mockWarnings, actualWarnings)
}

func (s *MeasuredAPITestSuite) TestLabelNames_Error() {
	mockError := errors.New("yikes u did bad")
	s.apiMock.On("LabelNames", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(s.addSomeLatency()).
		Return(nil, nil, mockError).Once()
	s.recorderMock.On("measure", "LabelNames", s.someLatency, mockError).Once()

	_, _, err := s.underTest.LabelNames(context.Background(), []string{}, time.Unix(0, 0), time.Unix(15, 0))

	s.Error(err)
}

func (s *MeasuredAPITestSuite) TestLabelValues_Success() {
	matches := []string{}
	label := "tag"
	startTime := time.Unix(0, 0)
	endTime := time.Unix(15, 0)
	mockValue := model.LabelValues{}
	mockWarnings := v1.Warnings{}
	s.apiMock.On("LabelValues", mock.Anything, label, matches, startTime, endTime).
		Run(s.addSomeLatency()).
		Return(mockValue, mockWarnings, nil).Once()
	s.recorderMock.On("measure", "LabelValues", s.someLatency, nil).Once()

	actualValue, actualWarnings, err := s.underTest.LabelValues(context.Background(), label, matches, startTime, endTime)

	s.NoError(err)
	s.Equal(mockValue, actualValue)
	s.Equal(mockWarnings, actualWarnings)
}

func (s *MeasuredAPITestSuite) TestLabelValues_Error() {
	mockError := errors.New("yikes u did bad")
	s.apiMock.On("LabelValues", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(s.addSomeLatency()).
		Return(nil, nil, mockError).Once()
	s.recorderMock.On("measure", "LabelValues", s.someLatency, mockError).Once()

	_, _, err := s.underTest.LabelValues(context.Background(), "tag", []string{}, time.Unix(0, 0), time.Unix(15, 0))

	s.Error(err)
}

func (s *MeasuredAPITestSuite) addSomeLatency() func(args mock.Arguments) {
	return func(args mock.Arguments) {
		s.now = s.now.Add(s.someLatency)
	}
}

func TestMeasuredClient(t *testing.T) {
	suite.Run(t, new(MeasuredAPITestSuite))
}
