package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func TestGatewayClientErrorBoundaryPreservesOpsEvidence(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 4)
	repo := &ingressRejectOpsRepo{}
	ops := service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.Use(GatewayClientErrorMessages(), OpsErrorLoggerMiddleware(ops))
	r.POST("/v1/responses", func(c *gin.Context) {
		service.SetOpsUpstreamError(c, 503, "original provider diagnostic", "")
		service.MarkOpsStreamFailure(c, "server_error", "server_error", "original terminal diagnostic", 503)
		c.Header("Content-Type", "text/event-stream")
		c.Header("Retry-After", "3")
		c.Header("X-Request-ID", "req-original")
		_, _ = fmt.Fprint(c.Writer, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"unchanged\"}\n\n")
		c.Writer.Flush()
		raw := "event: response.failed\ndata: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"server_error\",\"message\":\"original terminal diagnostic\"}}}\n\n"
		for _, part := range []string{raw[:31], raw[31:75], raw[75:]} {
			_, _ = c.Writer.Write([]byte(part))
		}
		c.Writer.Flush()
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/responses", nil))
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), "流内错误 503")
	require.Contains(t, w.Body.String(), `"delta":"unchanged"`)
	require.Equal(t, 1, strings.Count(w.Body.String(), "event: response.failed"))
	require.Equal(t, "3", w.Header().Get("Retry-After"))
	require.Equal(t, "req-original", w.Header().Get("X-Request-ID"))
	require.Len(t, opsErrorLogQueue, 1)
	job := <-opsErrorLogQueue
	require.Equal(t, "original terminal diagnostic", job.entry.ErrorMessage)
	require.Equal(t, 503, job.entry.StatusCode)
	require.NotNil(t, job.entry.UpstreamErrorMessage)
	require.Equal(t, "original terminal diagnostic", *job.entry.UpstreamErrorMessage)
	require.NotContains(t, job.entry.ErrorBody, "【")
}

func TestGatewayClientErrorOpsClassificationInvariant(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 4)
	ops := service.NewOpsService(&ingressRejectOpsRepo{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	run := func(enabled bool) *service.OpsInsertErrorLogInput {
		r := gin.New()
		if enabled {
			r.Use(GatewayClientErrorMessages())
		}
		r.Use(OpsErrorLoggerMiddleware(ops))
		r.POST("/v1/messages", func(c *gin.Context) {
			service.SetOpsUpstreamError(c, 403, "original upstream rejection", "")
			c.JSON(502, gin.H{"error": gin.H{"type": "upstream_error", "message": "Upstream access forbidden, please contact administrator"}})
		})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/messages", nil))
		if enabled {
			require.Contains(t, w.Body.String(), "上游认证异常")
		}
		require.Len(t, opsErrorLogQueue, 1)
		return (<-opsErrorLogQueue).entry
	}
	before, after := run(false), run(true)
	require.Equal(t, before.ErrorType, after.ErrorType)
	require.Equal(t, before.ErrorOwner, after.ErrorOwner)
	require.Equal(t, before.ErrorPhase, after.ErrorPhase)
	require.Equal(t, before.ErrorSource, after.ErrorSource)
	require.Equal(t, before.IsBusinessLimited, after.IsBusinessLimited)
	require.Equal(t, before.ErrorMessage, after.ErrorMessage)
	require.Equal(t, before.ErrorBody, after.ErrorBody)
	require.Equal(t, before.UpstreamStatusCode, after.UpstreamStatusCode)
	require.Equal(t, before.UpstreamErrorMessage, after.UpstreamErrorMessage)
}

func TestGatewayClientErrorPartialFlushAndMultiline(t *testing.T) {
	r := gin.New()
	r.Use(GatewayClientErrorMessages())
	prefix := "event: response.output_text.delta\ndata: {\"delta\":\"keep"
	end := " text\"}\n\n"
	failure := "id: original\r\nevent: error\r\ndata: {\"error\": {\"status_code\":503,\r\ndata: \"message\":\"internal\",\"code\":\"server_error\"}}\r\n\r\n"
	r.GET("/test", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(c.Writer, prefix)
		c.Writer.Flush()
		_, _ = fmt.Fprint(c.Writer, end+failure)
		c.Writer.Flush()
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))
	require.True(t, strings.HasPrefix(w.Body.String(), prefix+end))
	require.Contains(t, w.Body.String(), "流内错误 503")
	require.Contains(t, w.Body.String(), "id: original\r\nevent: error\r\ndata:")
	require.Equal(t, 2, strings.Count(w.Body.String(), "\r\ndata:"))
}

func TestGatewayClientErrorLargeSuccessDoesNotDisableLaterErrors(t *testing.T) {
	r := gin.New()
	r.Use(GatewayClientErrorMessages())
	prefix := "data: {\"delta\":\"" + strings.Repeat("x", maxGatewayClientErrorFrame)
	ending := "\"}\n\n"
	failure := "data: {\"error\":{\"code\":503,\"message\":\"internal\"}}\n\n"
	r.GET("/test", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(c.Writer, prefix)
		_, _ = fmt.Fprint(c.Writer, ending+failure)
		c.Writer.Flush()
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))
	require.True(t, strings.HasPrefix(w.Body.String(), prefix+ending))
	require.Contains(t, w.Body.String(), "流内错误 503")
}

type clientHintUpstream struct {
	service.HTTPUpstream
	calls int
	body  string
}

func (u *clientHintUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.calls++
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(u.body))}, nil
}

func TestGatewayClientErrorNativeAndPassthrough(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		t.Run(fmt.Sprint(passthrough), func(t *testing.T) {
			stream := "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_hint\",\"model\":\"gpt-5\"}}\n\n" +
				"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"keep model output\"}\n\n" +
				"event: error\ndata: {\"type\":\"error\",\"error\":{\"code\":\"server_is_overloaded\",\"message\":\"server is overloaded\"}}\n\n" +
				"event: response.failed\ndata: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_hint\",\"status\":\"failed\",\"error\":{\"code\":\"server_is_overloaded\",\"message\":\"server is overloaded\"}}}\n\n"
			run := func(enabled bool) (*httptest.ResponseRecorder, string, int) {
				upstream := &clientHintUpstream{body: stream}
				cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: 1024 * 1024}}
				svc := service.NewOpenAIGatewayService(nil, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, upstream, nil, nil, nil, nil, nil, nil, nil, nil)
				account := &service.Account{ID: 701, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "local-test-key", "base_url": "https://api.openai.com"}, Extra: map[string]any{"openai_passthrough": passthrough}}
				r := gin.New()
				if enabled {
					r.Use(GatewayClientErrorMessages())
				}
				r.Use(OpsErrorLoggerMiddleware(nil))
				internalError := ""
				r.POST("/v1/responses", func(c *gin.Context) {
					_, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5","input":"hello","stream":true}`))
					if err != nil {
						internalError = err.Error()
					}
				})
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/responses", nil))
				return w, internalError, upstream.calls
			}
			baseline, baseErr, baseCalls := run(false)
			rewritten, gotErr, gotCalls := run(true)
			require.Contains(t, rewritten.Body.String(), "【服务繁忙｜流内错误 503】")
			require.Equal(t, baseline.Code, rewritten.Code)
			require.Equal(t, baseErr, gotErr, "internal classification still sees original error")
			require.Equal(t, 1, baseCalls)
			require.Equal(t, baseCalls, gotCalls, "presentation does not retry")
			stripMessages := func(body string) string {
				lines := strings.Split(body, "\n")
				for i, line := range lines {
					if strings.HasPrefix(line, "data: ") {
						data := []byte(strings.TrimPrefix(line, "data: "))
						for _, path := range []string{"error.message", "response.error.message"} {
							if gjson.GetBytes(data, path).Exists() {
								data, _ = sjson.SetBytes(data, path, "<message>")
							}
						}
						lines[i] = "data: " + string(data)
					}
				}
				return strings.Join(lines, "\n")
			}
			require.Equal(t, stripMessages(baseline.Body.String()), stripMessages(rewritten.Body.String()), "event order, compatibility codes and model output stay identical")
		})
	}
}

func TestGatewayClientErrorBoundaryEnvelopes(t *testing.T) {
	for _, body := range []string{`{"error":{"type":"api_error","message":"internal"}}`, `{"type":"error","error":{"type":"api_error","message":"internal"}}`, `{"error":{"code":503,"message":"internal","status":"UNAVAILABLE"}}`} {
		t.Run(body, func(t *testing.T) {
			r := gin.New()
			r.Use(GatewayClientErrorMessages())
			r.POST("/test", func(c *gin.Context) { c.Data(503, "application/json", []byte(body)) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("POST", "/test", nil))
			require.Equal(t, 503, w.Code)
			require.Contains(t, gjson.Get(w.Body.String(), "error.message").String(), "【服务暂时不可用｜503】")
		})
	}
}

func TestGatewayClientErrorBoundaryPassesThrough(t *testing.T) {
	for _, tt := range []struct {
		name          string
		status        int
		content, body string
		custom        bool
	}{
		{"success", 200, "application/json", `{"output":[{"error":{"message":"fictional"}}]}`, false},
		{"success SSE", 200, "text/event-stream", "event: response.output_text.delta\ndata: {\"delta\":\"error\"}\n\n", false},
		{"malformed", 503, "application/json", `{"error":`, false},
		{"custom", 503, "application/json", `{"error":{"message":"administrator wording"}}`, true},
		{"html", 503, "text/html", `<html>unavailable</html>`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(GatewayClientErrorMessages())
			r.GET("/test", func(c *gin.Context) {
				if tt.custom {
					service.PreserveGatewayClientErrorMessage(c)
				}
				c.Data(tt.status, tt.content, []byte(tt.body))
				require.Equal(t, len(tt.body), c.Writer.Size())
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
			require.Equal(t, tt.body, w.Body.String())
		})
	}
}

func TestGatewayClientErrorBoundarySSEDataOnlyAndCRLF(t *testing.T) {
	for _, separator := range []string{"\n", "\r\n"} {
		r := gin.New()
		r.Use(GatewayClientErrorMessages())
		r.GET("/test", func(c *gin.Context) {
			c.Header("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(c.Writer, ": ping"+separator+separator+`data: {"error":{"code":503,"message":"internal","status":"UNAVAILABLE"}}`+separator+separator)
			c.Writer.Flush()
		})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))
		require.True(t, strings.HasPrefix(w.Body.String(), ": ping"+separator+separator))
		require.Contains(t, w.Body.String(), "流内错误 503")
		require.Equal(t, 2, strings.Count(w.Body.String(), separator+separator))
	}
}
