package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type canvasDeadlineUpstream struct{ target *url.URL }

func (u canvasDeadlineUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL = u.target
	return http.DefaultClient.Do(clone)
}
func (u canvasDeadlineUpstream) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, id, concurrency)
}

func TestManagedCanvasImageDeadlineClosesActualUpstreamConnection(t *testing.T) {
	disconnected := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
			close(disconnected)
		case <-time.After(5 * time.Second):
		}
	}))
	defer server.Close()
	target, err := url.Parse(server.URL)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), ctxkey.ManagedImageTask, true), 250*time.Millisecond)
	defer cancel()
	body := []byte(`{"model":"gpt-image-2","prompt":"timeout test"}`)
	c, _ := newOpenAIImagesTestContext(t, body)
	svc := newOpenAIImagesTestService(canvasDeadlineUpstream{target})
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	_, err = svc.ForwardImages(ctx, c, newOpenAIImagesAPIKeyAccount(), body, parsed, "")
	require.Error(t, err)
	select {
	case <-disconnected:
	case <-time.After(time.Second):
		t.Fatal("deadline did not close the upstream HTTP connection")
	}
}

func TestManagedCanvasContextRetainsServerDeadlineAcrossDetachHelpers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxkey.ManagedImageTask, true))
	upstream, release := detachUpstreamContext(ctx)
	defer release()
	stream, releaseStream := detachStreamUpstreamContext(ctx, true)
	defer releaseStream()
	cancel()
	require.ErrorIs(t, upstream.Err(), context.Canceled)
	require.ErrorIs(t, stream.Err(), context.Canceled)
}
