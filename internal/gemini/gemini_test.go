package gemini

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var step = Request{Instructions: "Read the request.", Prompt: "Request: hi", Schema: json.RawMessage(`{"type":"object"}`), MaxTokens: 100}

func testClient(t *testing.T, h http.HandlerFunc) (*Client, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	c := NewClient("test-key")
	c.endpoint = srv.URL + "/"
	return c, &calls
}

// reply is a generateContent response whose text is `text`, after a thought the caller shouldn't see.
func reply(text, finish string) string {
	parts, _ := json.Marshal([]part{{Text: "thinking…", Thought: true}, {Text: text}})
	return `{"candidates":[{"content":{"parts":` + string(parts) + `},"finishReason":"` + finish + `"}]}`
}

func TestGenerateFallsBackAndRestsTheFailedModel(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf("key not sent")
		}
		if strings.Contains(r.URL.Path, FreeTier[0].ID) {
			http.Error(w, `{"error":{"code":503,"status":"UNAVAILABLE"}}`, http.StatusServiceUnavailable)
			return
		}
		body, _ := io.ReadAll(r.Body)
		for _, want := range []string{`"responseJsonSchema":{"type":"object"}`, `"thinkingLevel":"MINIMAL"`, `"maxOutputTokens":100`} {
			if !strings.Contains(string(body), want) {
				t.Errorf("request body missing %s: %s", want, body)
			}
		}
		_, _ = io.WriteString(w, reply(`{"message":"hi"}`, "STOP"))
	})

	got, err := c.Generate(t.Context(), step)
	if err != nil || string(got) != `{"message":"hi"}` {
		t.Fatalf("got %s, %v", got, err)
	}
	if order := c.order(time.Now()); order[len(order)-1].ID != FreeTier[0].ID {
		t.Errorf("the busy model should go last, got %v", order)
	}
}

func TestQuotaWinsAndCarriesTheRetryDelay(t *testing.T) {
	c, calls := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, FreeTier[0].ID) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":{"code":429,"details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"36s"}]}}`)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	_, err := c.Generate(t.Context(), step)
	var e *Error
	if !errors.As(err, &e) || e.Kind != Quota || e.RetryAfter != 36*time.Second {
		t.Fatalf("got %+v", err)
	}
	if calls.Load() != int32(len(FreeTier)) {
		t.Errorf("every model should be tried, got %d calls", calls.Load())
	}
}

func TestBlockedAndBadKeyDontTryOtherModels(t *testing.T) {
	cases := map[Kind]http.HandlerFunc{
		Blocked: func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, reply(`{}`, "SAFETY")) },
		Unauthorized: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"code":400,"status":"INVALID_ARGUMENT","details":[{"reason":"API_KEY_INVALID"}]}}`)
		},
	}
	for kind, h := range cases {
		c, calls := testClient(t, h)
		_, err := c.Generate(t.Context(), step)
		var e *Error
		if !errors.As(err, &e) || e.Kind != kind || calls.Load() != 1 {
			t.Errorf("kind %d: got %+v after %d calls", kind, err, calls.Load())
		}
	}
}

func TestCutOffReplyIsUnavailable(t *testing.T) {
	c, calls := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, reply(`{"message":"hal`, "MAX_TOKENS"))
	})
	_, err := c.Generate(t.Context(), step)
	var e *Error
	if !errors.As(err, &e) || e.Kind != Unavailable || calls.Load() != int32(len(FreeTier)) {
		t.Fatalf("got %+v after %d calls", err, calls.Load())
	}
}
