package sse

import (
	"context"
	"fmt"
	"net/http"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

// Writer writes StreamEvents to an HTTP response in text/event-stream format.
type Writer struct{}

// Write consumes events from the channel and writes them as SSE to the ResponseWriter.
// translateFn converts each domain event to the JSON bytes to send. If it returns nil,
// the event is skipped. After the channel closes, "data: [DONE]" is sent.
func (w Writer) Write(
	ctx context.Context,
	rw http.ResponseWriter,
	events <-chan vo.StreamEvent,
	translateFn func(vo.StreamEvent) []byte,
) {
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")
	rw.WriteHeader(http.StatusOK)

	flusher, canFlush := rw.(http.Flusher)

	for event := range events {
		if ctx.Err() != nil {
			return
		}

		chunk := translateFn(event)
		if chunk == nil {
			continue
		}

		fmt.Fprintf(rw, "data: %s\n\n", chunk)
		if canFlush {
			flusher.Flush()
		}
	}

	fmt.Fprint(rw, "data: [DONE]\n\n")
	if canFlush {
		flusher.Flush()
	}
}
