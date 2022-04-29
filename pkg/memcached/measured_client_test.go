package memcached

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/suite"

	"github.com/bradfitz/gomemcache/memcache"
)

type MeasuredClientTestSuite struct {
	suite.Suite

	someKey     string
	someItem    *memcache.Item
	now         time.Time
	someLatency time.Duration
	timeNow     func() time.Time

	recorderMock   *MockRecorder
	clientMock     *MockClient
	measuredClient Client
}

func (s *MeasuredClientTestSuite) SetupTest() {
	s.someKey = "foo"
	s.someItem = &memcache.Item{
		Key:        "foo",
		Value:      []byte("bar"),
		Flags:      42,
		Expiration: 288,
	}
	s.now = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	s.timeNow = func() time.Time { return s.now }
	s.someLatency = 10 * time.Millisecond

	s.recorderMock = &MockRecorder{}
	s.clientMock = &MockClient{}

	s.measuredClient = NewMeasuredClient(s.clientMock, s.recorderMock, s.timeNow)
}

func (s *MeasuredClientTestSuite) TearDownTest() {
	s.recorderMock.AssertExpectations(s.T())
	s.clientMock.AssertExpectations(s.T())
}

func (s *MeasuredClientTestSuite) TestGet_success() {
	s.clientMock.On("Get", s.someKey).
		Run(s.addSomeLatency()).
		Return(s.someItem, nil).Once()
	s.recorderMock.On("measure", "Get", s.someLatency, nil)

	item, err := s.measuredClient.Get(s.someKey)
	s.NoError(err)
	s.Equal(s.someItem, item)
}

func (s *MeasuredClientTestSuite) TestGet_error() {
	s.clientMock.On("Get", s.someKey).
		Run(s.addSomeLatency()).
		Return(s.someItem, memcache.ErrCacheMiss).Once()
	s.recorderMock.On("measure", "Get", s.someLatency, memcache.ErrCacheMiss)

	_, err := s.measuredClient.Get(s.someKey)
	s.Equal(memcache.ErrCacheMiss, err)
}

func (s *MeasuredClientTestSuite) TestAdd_success() {
	s.clientMock.On("Add", s.someItem).
		Run(s.addSomeLatency()).
		Return(nil).Once()
	s.recorderMock.On("measure", "Add", s.someLatency, nil)

	err := s.measuredClient.Add(s.someItem)
	s.NoError(err)
}

func (s *MeasuredClientTestSuite) TestAdd_error() {
	s.clientMock.On("Add", s.someItem).
		Run(s.addSomeLatency()).
		Return(memcache.ErrCacheMiss).Once()
	s.recorderMock.On("measure", "Add", s.someLatency, memcache.ErrCacheMiss)

	err := s.measuredClient.Add(s.someItem)
	s.Equal(memcache.ErrCacheMiss, err)
}

func (s *MeasuredClientTestSuite) TestSet_success() {
	s.clientMock.On("Set", s.someItem).
		Run(s.addSomeLatency()).
		Return(nil).Once()
	s.recorderMock.On("measure", "Set", s.someLatency, nil)

	err := s.measuredClient.Set(s.someItem)
	s.NoError(err)
}

func (s *MeasuredClientTestSuite) TestSet_error() {
	s.clientMock.On("Set", s.someItem).
		Run(s.addSomeLatency()).
		Return(memcache.ErrCacheMiss).Once()
	s.recorderMock.On("measure", "Set", s.someLatency, memcache.ErrCacheMiss)

	err := s.measuredClient.Set(s.someItem)
	s.Equal(memcache.ErrCacheMiss, err)
}

func (s *MeasuredClientTestSuite) TestCompareAndSwap_success() {
	s.clientMock.On("CompareAndSwap", s.someItem).
		Run(s.addSomeLatency()).
		Return(nil).Once()
	s.recorderMock.On("measure", "CompareAndSwap", s.someLatency, nil)

	err := s.measuredClient.CompareAndSwap(s.someItem)
	s.NoError(err)
}

func (s *MeasuredClientTestSuite) TestCompareAndSwap_error() {
	s.clientMock.On("CompareAndSwap", s.someItem).
		Run(s.addSomeLatency()).
		Return(memcache.ErrCacheMiss).Once()
	s.recorderMock.On("measure", "CompareAndSwap", s.someLatency, memcache.ErrCacheMiss)

	err := s.measuredClient.CompareAndSwap(s.someItem)
	s.Equal(memcache.ErrCacheMiss, err)
}

func (s *MeasuredClientTestSuite) TestDelete_success() {
	s.clientMock.On("Delete", s.someKey).
		Run(s.addSomeLatency()).
		Return(nil).Once()
	s.recorderMock.On("measure", "Delete", s.someLatency, nil)

	err := s.measuredClient.Delete(s.someKey)
	s.NoError(err)
}

func (s *MeasuredClientTestSuite) TestDelete_error() {
	s.clientMock.On("Delete", s.someKey).
		Run(s.addSomeLatency()).
		Return(memcache.ErrCacheMiss).Once()
	s.recorderMock.On("measure", "Delete", s.someLatency, memcache.ErrCacheMiss)

	err := s.measuredClient.Delete(s.someKey)
	s.Equal(memcache.ErrCacheMiss, err)
}

func (s *MeasuredClientTestSuite) addSomeLatency() func(args mock.Arguments) {
	return func(args mock.Arguments) {
		s.now = s.now.Add(s.someLatency)
	}
}

func TestMeasuredClient(t *testing.T) {
	suite.Run(t, new(MeasuredClientTestSuite))
}
