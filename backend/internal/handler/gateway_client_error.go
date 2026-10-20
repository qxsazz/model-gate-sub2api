package handler

import (
	"bytes"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const maxGatewayClientErrorFrame = 256 * 1024

// GatewayClientErrorMessages belongs ONLY to inference gateway route chains,
// before OpsErrorLoggerMiddleware and authentication. Ops therefore captures
// the original bytes; callers still see their original write counts/Size.
// No classification/recovery input is changed by this last-mile presentation.
func GatewayClientErrorMessages() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Account reporting endpoints are not model invocation errors.
		if path := c.FullPath(); strings.HasSuffix(path, "/sub2api/billing") || strings.HasSuffix(path, "/usage") {
			c.Next()
			return
		}
		original := c.Writer
		w := &gatewayClientErrorWriter{ResponseWriter: original, ctx: c}
		w.size.Store(-1)
		c.Writer = w
		defer func() {
			w.mu.Lock()
			if len(w.pending) > 0 {
				_, _ = original.Write(w.pending)
				w.pending = nil
			}
			w.mu.Unlock()
			c.Writer = original
		}()
		c.Next()
	}
}

type gatewayClientErrorWriter struct {
	gin.ResponseWriter
	ctx       *gin.Context
	mu        sync.Mutex
	size      atomic.Int64
	pending   []byte
	disabled  bool
	skipFrame bool
	rawTail   []byte
}

func (w *gatewayClientErrorWriter) Size() int {
	size := w.size.Load()
	if size < 0 && w.Written() {
		return 0
	}
	return int(size)
}

func (w *gatewayClientErrorWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

func (w *gatewayClientErrorWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Preserve the original commit point, even for a fragmented error frame.
	w.WriteHeaderNow()
	w.size.CompareAndSwap(-1, 0)
	n := len(p)
	contentType := strings.ToLower(w.Header().Get("Content-Type"))
	sse := strings.HasPrefix(contentType, "text/event-stream")
	jsonError := w.Status() >= 400 && strings.Contains(contentType, "json")
	// An explicitly fixed-length response cannot change size without changing a
	// header; leave it alone. Standard gateway writers do not set Content-Length.
	if w.disabled || w.Header().Get("Content-Encoding") != "" || w.Header().Get("Content-Length") != "" || (!sse && !jsonError) {
		written, err := w.ResponseWriter.Write(p)
		w.size.Add(int64(written))
		return written, err
	}
	if sse && w.skipFrame {
		probe := append(append([]byte(nil), w.rawTail...), p...)
		end, delimiter := gatewaySSEBoundary(probe)
		if end < 0 {
			w.rememberRawTail(probe)
			written, err := w.ResponseWriter.Write(p)
			w.size.Add(int64(written))
			return written, err
		}
		consumed := end + delimiter - len(w.rawTail)
		if _, err := w.ResponseWriter.Write(p[:consumed]); err != nil {
			return 0, err
		}
		p = p[consumed:]
		w.skipFrame = false
		w.rawTail = nil
	}
	w.pending = append(w.pending, p...)
	if !sse && len(w.pending) > maxGatewayClientErrorFrame {
		w.disabled = true
		_, err := w.ResponseWriter.Write(w.pending)
		w.pending = nil
		if err != nil {
			return 0, err
		}
		w.size.Add(int64(n))
		return n, nil
	}
	if !sse {
		if gjson.ValidBytes(w.pending) {
			payload := service.RewriteGatewayClientError(w.ctx, w.pending, w.Status(), "")
			_, err := w.ResponseWriter.Write(payload)
			w.pending = nil
			if err != nil {
				return 0, err
			}
		}
	} else {
		for {
			end, delimiter := gatewaySSEBoundary(w.pending)
			if end < 0 {
				break
			}
			frame := w.pending[:end+delimiter]
			rewritten := rewriteGatewayClientSSEFrame(w.ctx, frame, w.Status())
			if _, err := w.ResponseWriter.Write(rewritten); err != nil {
				return 0, err
			}
			w.pending = w.pending[end+delimiter:]
		}
		if len(w.pending) == 0 {
			w.pending = nil
		} else if len(w.pending) > maxGatewayClientErrorFrame {
			w.skipFrame = true
			w.rememberRawTail(w.pending)
			if _, err := w.ResponseWriter.Write(w.pending); err != nil {
				return 0, err
			}
			w.pending = nil
		}
	}
	w.size.Add(int64(n))
	return n, nil
}

func (w *gatewayClientErrorWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Do not hold up an explicit flush of a partial/unknown frame. Once bytes
	// have been exposed we cannot safely reinterpret or replay that frame.
	if len(w.pending) > 0 {
		if strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
			w.skipFrame = true
			w.rememberRawTail(w.pending)
		} else {
			w.disabled = true
		}
		_, _ = w.ResponseWriter.Write(w.pending)
		w.pending = nil
	}
	w.ResponseWriter.Flush()
}

func gatewaySSEBoundary(p []byte) (int, int) {
	end, delimiter := bytes.Index(p, []byte("\n\n")), 2
	if crlf := bytes.Index(p, []byte("\r\n\r\n")); crlf >= 0 && (end < 0 || crlf < end) {
		return crlf, 4
	}
	return end, delimiter
}

func (w *gatewayClientErrorWriter) rememberRawTail(p []byte) {
	start := len(p) - 3
	if start < 0 {
		start = 0
	}
	w.rawTail = append(w.rawTail[:0], p[start:]...)
}

// Only protocol error envelopes are eligible. Preserve event names, comments,
// id/retry fields, newline style and every non-message JSON field verbatim.
func rewriteGatewayClientSSEFrame(c *gin.Context, frame []byte, status int) []byte {
	if len(frame) > maxGatewayClientErrorFrame {
		return frame
	}
	if !bytes.Contains(frame, []byte(`"error"`)) && !bytes.Contains(frame, []byte(`"response.failed"`)) && !bytes.Contains(frame, []byte("event: error")) && !bytes.Contains(frame, []byte("event: response.failed")) {
		return frame
	}
	type segment struct{ dataStart, frameStart, length int }
	var segments []segment
	var payload []byte
	event := ""
	offset := 0
	for _, line := range bytes.SplitAfter(frame, []byte("\n")) {
		text := bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r"))
		if bytes.HasPrefix(text, []byte("event:")) {
			event = strings.TrimSpace(string(text[6:]))
		}
		if bytes.HasPrefix(text, []byte("data:")) {
			start := 5
			if len(text) > 5 && text[5] == ' ' {
				start++
			}
			if len(segments) > 0 {
				payload = append(payload, '\n')
			}
			segments = append(segments, segment{len(payload), offset + start, len(text) - start})
			payload = append(payload, text[start:]...)
		}
		offset += len(line)
	}
	if len(segments) == 0 {
		return frame
	}
	rewritten := service.RewriteGatewayClientError(c, payload, status, event)
	if bytes.Equal(rewritten, payload) {
		return frame
	}
	// Change only the JSON string token, even when data spans multiple SSE
	// lines. JSON strings cannot contain literal newlines, so a valid message
	// token is wholly within one data line. All SSE framing remains verbatim.
	for _, path := range []string{"response.error.message", "error.message", "message"} {
		before, after := gjson.GetBytes(payload, path), gjson.GetBytes(rewritten, path)
		if before.Type != gjson.String || after.Type != gjson.String || before.Raw == after.Raw {
			continue
		}
		for _, part := range segments {
			if before.Index >= part.dataStart && before.Index+len(before.Raw) <= part.dataStart+part.length {
				start := part.frameStart + before.Index - part.dataStart
				out := make([]byte, 0, len(frame)+len(after.Raw)-len(before.Raw))
				out = append(out, frame[:start]...)
				out = append(out, after.Raw...)
				out = append(out, frame[start+len(before.Raw):]...)
				return out
			}
		}
	}
	return frame
}

var _ gin.ResponseWriter = (*gatewayClientErrorWriter)(nil)
var _ http.Flusher = (*gatewayClientErrorWriter)(nil)
