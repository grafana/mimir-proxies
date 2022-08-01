package remotewrite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grafana/mimir-proxies/pkg/remotewrite/remotewritemock"
	"github.com/grafana/mimir/pkg/mimirpb"

	"github.com/opentracing/opentracing-go"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type MeasuredClientTestSuite struct {
	suite.Suite

	now         time.Time
	someLatency time.Duration
	timeNow     func() time.Time

	recorderMock *MockRecorder
	clientMock   *remotewritemock.Client
	underTest    Client
}

func (s *MeasuredClientTestSuite) SetupTest() {
	s.now = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	s.timeNow = func() time.Time { return s.now }
	s.someLatency = 10 * time.Millisecond

	s.recorderMock = &MockRecorder{}
	s.clientMock = &remotewritemock.Client{}

	s.underTest = NewMeasuredClient(s.clientMock, s.recorderMock, opentracing.NoopTracer{}, s.timeNow)
}

func (s *MeasuredClientTestSuite) TearDownTest() {
	s.recorderMock.AssertExpectations(s.T())
	s.clientMock.AssertExpectations(s.T())
}

func (s *MeasuredClientTestSuite) TestQueryRange_Success() {
	writeRequest := &mimirpb.WriteRequest{}
	s.clientMock.On("Write", mock.Anything, writeRequest).
		Run(s.addSomeLatency()).
		Return(nil).Once()
	s.recorderMock.On("measure", "Write", s.someLatency, nil).Once()

	err := s.underTest.Write(context.Background(), writeRequest)

	s.NoError(err)
}

func (s *MeasuredClientTestSuite) TestQueryRange_Error() {
	writeRequest := &mimirpb.WriteRequest{}
	mockErr := errors.New("oom")
	s.clientMock.On("Write", mock.Anything, mock.Anything).
		Run(s.addSomeLatency()).
		Return(mockErr).Once()
	s.recorderMock.On("measure", "Write", s.someLatency, mockErr).Once()

	err := s.underTest.Write(context.Background(), writeRequest)

	s.Error(err)
}

func (s *MeasuredClientTestSuite) addSomeLatency() func(args mock.Arguments) {
	return func(args mock.Arguments) {
		s.now = s.now.Add(s.someLatency)
	}
}

func TestMeasuredClient(t *testing.T) {
	suite.Run(t, new(MeasuredClientTestSuite))
}
