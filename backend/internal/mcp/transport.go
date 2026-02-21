package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Event string
	Data  string
	ID    string
	Retry int
}

// SSEReader reads Server-Sent Events from an HTTP response
type SSEReader struct {
	reader *bufio.Reader
}

// NewSSEReader creates a new SSE reader
func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{
		reader: bufio.NewReader(r),
	}
}

// ReadEvent reads the next SSE event
func (r *SSEReader) ReadEvent() (*SSEEvent, error) {
	event := &SSEEvent{}

	for {
		line, err := r.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if event.Data != "" {
					return event, nil
				}
				return nil, err
			}
			return nil, err
		}

		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		if line == "" {
			if event.Data != "" || event.Event != "" {
				// Remove trailing newline from data
				event.Data = strings.TrimSuffix(event.Data, "\n")
				return event, nil
			}
			continue
		}

		if strings.HasPrefix(line, ":") {
			// Comment, ignore
			continue
		}

		colonIdx := strings.Index(line, ":")
		var field, value string
		if colonIdx == -1 {
			field = line
			value = ""
		} else {
			field = line[:colonIdx]
			value = line[colonIdx+1:]
			if strings.HasPrefix(value, " ") {
				value = value[1:]
			}
		}

		switch field {
		case "event":
			event.Event = value
		case "data":
			event.Data += value + "\n"
		case "id":
			event.ID = value
		case "retry":
			fmt.Sscanf(value, "%d", &event.Retry)
		}
	}
}

// SSETransport handles SSE connections to upstream MCP servers
type SSETransport struct {
	client   *http.Client
	url      string
	headers  map[string]string
}

// NewSSETransport creates a new SSE transport
func NewSSETransport(url string, headers map[string]string) *SSETransport {
	return &SSETransport{
		client:  &http.Client{},
		url:     url,
		headers: headers,
	}
}

// Connect establishes an SSE connection and returns a channel of events
func (t *SSETransport) Connect(ctx context.Context) (<-chan *SSEEvent, <-chan error, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	events := make(chan *SSEEvent, 10)
	errors := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errors)
		defer resp.Body.Close()

		reader := NewSSEReader(resp.Body)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				event, err := reader.ReadEvent()
				if err != nil {
					if err != io.EOF {
						errors <- err
					}
					return
				}
				events <- event
			}
		}
	}()

	return events, errors, nil
}

// SSEWriter writes Server-Sent Events to an HTTP response
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter creates a new SSE writer
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Write status and flush headers immediately
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	return &SSEWriter{
		w:       w,
		flusher: flusher,
	}, nil
}

// WriteEvent writes an SSE event
func (w *SSEWriter) WriteEvent(event *SSEEvent) error {
	if event.Event != "" {
		if _, err := fmt.Fprintf(w.w, "event: %s\n", event.Event); err != nil {
			return err
		}
	}

	if event.ID != "" {
		if _, err := fmt.Fprintf(w.w, "id: %s\n", event.ID); err != nil {
			return err
		}
	}

	if event.Retry > 0 {
		if _, err := fmt.Fprintf(w.w, "retry: %d\n", event.Retry); err != nil {
			return err
		}
	}

	// Split data by newlines and write each line
	lines := strings.Split(event.Data, "\n")
	for _, line := range lines {
		if _, err := fmt.Fprintf(w.w, "data: %s\n", line); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprint(w.w, "\n"); err != nil {
		return err
	}

	w.flusher.Flush()
	return nil
}

// WriteMessage writes a JSON-RPC message as an SSE event
func (w *SSEWriter) WriteMessage(eventType string, msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return w.WriteEvent(&SSEEvent{
		Event: eventType,
		Data:  string(data),
	})
}

// WriteJSONRPCResponse writes a JSON-RPC response as an SSE event
func (w *SSEWriter) WriteJSONRPCResponse(resp *JSONRPCResponse) error {
	return w.WriteMessage("message", resp)
}

// WriteJSONRPCNotification writes a JSON-RPC notification as an SSE event
func (w *SSEWriter) WriteJSONRPCNotification(notification *JSONRPCNotification) error {
	return w.WriteMessage("message", notification)
}

// WriteEndpoint writes the endpoint event (used during SSE initialization)
func (w *SSEWriter) WriteEndpoint(endpoint string) error {
	return w.WriteEvent(&SSEEvent{
		Event: "endpoint",
		Data:  endpoint,
	})
}

// Close writes the close event
func (w *SSEWriter) Close() error {
	return w.WriteEvent(&SSEEvent{
		Event: "close",
	})
}

// ProxySSE proxies SSE events from an upstream server to a client
func ProxySSE(ctx context.Context, writer *SSEWriter, upstream <-chan *SSEEvent, errors <-chan error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errors:
			if err != nil {
				log.Error().Err(err).Msg("Upstream SSE error")
				return err
			}
		case event, ok := <-upstream:
			if !ok {
				return nil
			}
			if err := writer.WriteEvent(event); err != nil {
				return err
			}
		}
	}
}
