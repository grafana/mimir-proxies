package apiwrite

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	ingestermock "github.com/grafana/mimir-proxies/pkg/datadog/ingester/ingestermock"

	"github.com/grafana/mimir-proxies/pkg/datadog/ddstructs"

	"github.com/grafana/mimir-proxies/pkg/server/middleware"

	"github.com/gorilla/mux"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/user"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/grafana/mimir-proxies/pkg/ctxlog"
	"github.com/grafana/mimir-proxies/pkg/errorx"
	"github.com/grafana/mimir-proxies/pkg/server"
)

var errMap = map[string]struct {
	err            error
	expectedStatus int
}{
	"canceled": {
		err:            context.Canceled,
		expectedStatus: httpStatusCanceled,
	},
	"disabled": {
		err:            errorx.Disabled{},
		expectedStatus: http.StatusNotImplemented,
	},
	"unimplemented": {
		err:            errorx.Unimplemented{},
		expectedStatus: http.StatusNotImplemented,
	},
	"badrequest": {
		err:            errorx.BadRequest{},
		expectedStatus: http.StatusBadRequest,
	},
	"badrequest.wrapping.internal": {
		err:            errorx.BadRequest{Err: errorx.Internal{}},
		expectedStatus: http.StatusBadRequest,
	},
	"internal": {
		err:            errorx.Internal{},
		expectedStatus: http.StatusInternalServerError,
	},
	"internal.wrapping.bad.mrequest": {
		err:            errorx.Internal{Err: errorx.BadRequest{}},
		expectedStatus: http.StatusInternalServerError,
	},
	"unmapped": {
		err:            context.DeadlineExceeded,
		expectedStatus: http.StatusInternalServerError,
	},
}

type APISuite struct {
	suite.Suite

	apiw   *API
	srv    *server.Server
	srvCfg server.Config

	ingesterMock *ingestermock.Ingester

	shutdownF func(error)
}

func (s *APISuite) SetupTest() {
	s.srvCfg = server.Config{
		HTTPListenAddress: "127.0.0.1",
		HTTPListenPort:    0, // Request system available port
	}

	s.ingesterMock = &ingestermock.Ingester{}

	apiConfigWrite := Config{}

	var err error
	s.apiw = NewAPI(apiConfigWrite, ctxlog.NewProvider(log.NewNopLogger()), s.ingesterMock, opentracing.NoopTracer{})

	authMiddleware := middleware.HTTPFakeAuth{}

	s.srv, err = server.NewServer(log.NewNopLogger(), s.srvCfg, mux.NewRouter(), []middleware.Interface{authMiddleware})
	s.Require().NoError(err)
	s.apiw.RegisterAll(s.srv.Router)

	s.shutdownF = s.srv.Shutdown

	go func() {
		err := s.srv.Run()
		s.Require().NoError(err)
	}()

	// Wait until the server is up before returning.
	for {
		resp := s.get("/")
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (s *APISuite) TearDownTest() {
	s.ingesterMock.AssertExpectations(s.T())

	s.shutdownF(nil)
	defaultRegistry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = defaultRegistry
	prometheus.DefaultGatherer = defaultRegistry
}

func (s *APISuite) TestSeriesPush_HappyCase() {
	series := ddstructs.Series{
		{
			Name:   "foo",
			Points: []ddstructs.Point{{Ts: 100, Value: 200}},
		},
	}

	s.ingesterMock.On("StoreMetrics", contextWithValues(), series).
		Once().
		Return(nil)

	resp := s.post("/api/v1/series", SeriesPushPayload{Series: series})
	s.assertStatusAndCloseBody(http.StatusOK, resp)
	s.ingesterMock.AssertExpectations(s.T())
}

func (s *APISuite) TestSeriesPush_FloatsAsStrings() {
	type anything interface{}
	type object map[string]anything
	type array []anything
	seriesPushPayloadWithFloatsAsString := object{
		"series": []object{
			{
				"metric": "foo",
				"points": array{
					array{"100", "200.5"},
				},
			},
		},
	}
	expectedSeries := ddstructs.Series{
		{
			Name:   "foo",
			Points: []ddstructs.Point{{Ts: 100, Value: 200.5}},
		},
	}

	s.ingesterMock.On("StoreMetrics", contextWithValues(), expectedSeries).
		Once().
		Return(nil)

	resp := s.post("/api/v1/series", seriesPushPayloadWithFloatsAsString)
	s.assertStatusAndCloseBody(http.StatusOK, resp)
	s.ingesterMock.AssertExpectations(s.T())
}

func (s *APISuite) TestSeriesPush_DeflateEncoded() {
	series := ddstructs.Series{
		{
			Name:   "foo",
			Points: []ddstructs.Point{{Ts: 100, Value: 200}},
		},
	}

	s.ingesterMock.On("StoreMetrics", contextWithValues(), series).
		Once().
		Return(nil)

	resp := s.postDeflateEncoded("/api/v1/series", SeriesPushPayload{Series: series})
	s.assertStatusAndCloseBody(http.StatusOK, resp)
	s.ingesterMock.AssertExpectations(s.T())
}

func (s *APISuite) TestSeriesPush_DeflateEncodingBroken() {
	req, err := http.NewRequest(
		http.MethodPost,
		s.baseURL()+"/api/v1/series",
		bytes.NewBuffer([]byte("this is not zlib")),
	)
	s.Require().NoError(err)
	req.Header.Add("Content-Encoding", "deflate")

	s.assertStatusAndCloseBody(http.StatusBadRequest, s.do(req))
	s.ingesterMock.AssertExpectations(s.T())
}

func (s *APISuite) TestCheckRun_TestSeriesPush_MalformedJSONMalformedJSON() {
	s.assertStatusAndCloseBody(http.StatusBadRequest, s.postMalformedJSON("/api/v1/series"))
	s.ingesterMock.AssertExpectations(s.T())
}

func (s *APISuite) TestSeriesPush_Errors() {
	for slug, res := range errMap {
		s.ingesterMock.On("StoreMetrics", contextWithValues(), ddstructs.Series{{Name: slug}}).
			Once().
			Return(res.err)
	}

	for slug, res := range errMap {
		s.Run(slug, func() {
			resp := s.post("/api/v1/series", SeriesPushPayload{Series: ddstructs.Series{{Name: slug}}})
			s.assertStatusAndCloseBody(res.expectedStatus, resp)
		})
	}

	s.ingesterMock.AssertExpectations(s.T())
}

func (s *APISuite) TestIntake_HappyCase() {
	const hostName = "localhost"
	systemTags := []string{"foo:bar"}

	s.ingesterMock.On("StoreHostTags", contextWithValues(), hostName, systemTags).
		Once().
		Return(nil)

	resp := s.post("/intake/", ddstructs.Payload{
		InternalHostname: hostName,
		HostTags: &ddstructs.Tags{
			System: systemTags,
		},
	})
	s.assertStatusAndCloseBody(http.StatusOK, resp)

	s.ingesterMock.AssertExpectations(s.T())
}

func (s *APISuite) TestIntake_NoTags() {
	hostName := "localhost"

	resp := s.post("/intake/", ddstructs.Payload{
		InternalHostname: hostName,
		HostTags:         nil,
	})
	s.assertStatusAndCloseBody(http.StatusOK, resp)

	s.ingesterMock.AssertNotCalled(s.T(), "StoreHostTags")
}

func (s *APISuite) TestIntake_Errors() {
	for slug, res := range errMap {
		s.ingesterMock.On("StoreHostTags", contextWithValues(), slug, mock.Anything).
			Once().
			Return(res.err)
	}

	for slug, res := range errMap {
		s.Run(slug, func() {
			payload := ddstructs.Payload{
				InternalHostname: slug,
				HostTags: &ddstructs.Tags{
					System: []string{"foo:bar"},
				},
			}
			resp := s.post("/intake/", payload)
			s.assertStatusAndCloseBody(res.expectedStatus, resp)
		})
	}

	s.ingesterMock.AssertExpectations(s.T())
}

func (s *APISuite) TestIntake_MalformedJSON() {
	s.assertStatusAndCloseBody(http.StatusBadRequest, s.postMalformedJSON("/intake/"))
	s.ingesterMock.AssertExpectations(s.T())
}

func (s *APISuite) TestCheckRun_HappyCase() {
	checks := ddstructs.ServiceChecks{{CheckName: "foo"}}

	s.ingesterMock.On("StoreCheckRun", contextWithValues(), checks).
		Once().
		Return(nil)

	resp := s.post("/api/v1/check_run", checks)
	s.assertStatusAndCloseBody(http.StatusOK, resp)

	s.ingesterMock.AssertExpectations(s.T())
}

func (s *APISuite) TestCheckRun_Errors() {
	for slug, res := range errMap {
		s.ingesterMock.On("StoreCheckRun", contextWithValues(), ddstructs.ServiceChecks{{CheckName: slug}}).
			Once().
			Return(res.err)
	}

	for slug, res := range errMap {
		s.Run(slug, func() {
			resp := s.post("/api/v1/check_run", ddstructs.ServiceChecks{{CheckName: slug}})
			s.assertStatusAndCloseBody(res.expectedStatus, resp)
		})
	}

	s.ingesterMock.AssertExpectations(s.T())
}

func (s *APISuite) TestCheckRun_MalformedJSON() {
	resp := s.postMalformedJSON("/api/v1/check_run")
	s.assertStatusAndCloseBody(http.StatusBadRequest, resp)
	s.ingesterMock.AssertExpectations(s.T())
}

func (s *APISuite) TestValidate() {
	// Common to read and write
	got := s.getJSON("/api/v1/validate")
	s.Equal(map[string]interface{}{}, got)
}

func (s *APISuite) TestSketches() {
	// Common to read and write
	// Should be not found in both read and write
	resp := s.post("/api/beta/sketches", struct{}{})
	s.assertStatusAndCloseBody(http.StatusNotFound, resp)

	resp = s.post("/api/v1/sketches", struct{}{})
	s.assertStatusAndCloseBody(http.StatusNotFound, resp)
}

func (s *APISuite) TestNotFoundRoute() {
	// Common to read and write
	s.assertStatusAndCloseBody(http.StatusNotFound, s.get("/foobar"))
}

func (s *APISuite) assertStatusAndCloseBody(expectedStatus int, resp *http.Response) {
	s.T().Helper()
	s.Equal(expectedStatus, resp.StatusCode, "Status code was not %d, it was %d (%s)", expectedStatus, resp.StatusCode, resp.Status)
	s.NoError(resp.Body.Close())
}

func (s *APISuite) post(path string, body interface{}) *http.Response {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(body)
	s.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, s.baseURL()+path, &buf)
	s.Require().NoError(err)

	return s.do(req)
}

func (s *APISuite) postDeflateEncoded(path string, body interface{}) *http.Response {
	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	err := json.NewEncoder(writer).Encode(body)
	s.Require().NoError(err)
	s.Require().NoError(writer.Close())

	req, err := http.NewRequest(http.MethodPost, s.baseURL()+path, &buf)
	s.Require().NoError(err)

	req.Header.Add("Content-Encoding", "deflate")

	return s.do(req)
}

func (s *APISuite) postMalformedJSON(path string) *http.Response {
	req, err := http.NewRequest(
		http.MethodPost,
		s.baseURL()+path,
		bytes.NewBuffer([]byte("this is not a json")),
	)
	s.Require().NoError(err)

	resp := s.do(req)
	return resp
}

func (s *APISuite) get(path string) *http.Response {
	req, err := http.NewRequest(http.MethodGet, s.baseURL()+path, http.NoBody)
	s.Require().NoError(err)
	return s.do(req)
}

func (s *APISuite) getJSON(path string) map[string]interface{} {
	resp := s.get(path)
	defer func() {
		_ = resp.Body.Close()
	}()
	if !s.Equal([]string{"application/json"}, resp.Header["Content-Type"]) {
		allBody, _ := io.ReadAll(resp.Body)
		s.FailNow("Response was not a json", "Body: %s", string(allBody))
	}

	val := make(map[string]interface{})
	err := json.NewDecoder(resp.Body).Decode(&val)
	s.Require().NoError(err)
	return val
}

func (s *APISuite) do(req *http.Request) *http.Response {
	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)

	return resp
}

func (s *APISuite) baseURL() string {
	return fmt.Sprintf("http://%s", s.srv.Addr())
}

func TestAPISuite(t *testing.T) {
	suite.Run(t, new(APISuite))
}

// contextWithValues provides a matcher for "github.com/stretchr/testify/mock" that matches
// context arguments with any log baggage and the org ID
func contextWithValues() interface{} {
	return mock.MatchedBy(func(ctx context.Context) bool {
		values := ctxlog.BaggageFrom(ctx)
		if len(values) == 0 {
			return false
		}
		_, err := user.ExtractOrgID(ctx)
		return err == nil
	})
}
