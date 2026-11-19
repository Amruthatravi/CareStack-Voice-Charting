package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func mock(t *testing.T, status int, content string, calls *int32) *Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(calls, 1)
		if status == 429 {
			w.Header().Set("Retry-After", "60")
		}
		w.WriteHeader(status)
		w.Write([]byte(`{"choices":[{"message":{"content":` + content + `}}]}`))
	}))
	t.Cleanup(srv.Close)
	return New(Config{APIKey: "k", Model: "m", BaseURL: srv.URL, Timeout: time.Second, RPM: 20, MinGap: 0})
}

func TestInterpretAndCache(t *testing.T) {
	var n int32
	c := mock(t, 200, `"{\"action\":\"set_pd\",\"tooth\":null,\"site\":null,\"value\":5,\"flag\":null}"`, &n)
	h := Hint{CursorTooth: 3, HasLast: true, LastTooth: 2, LastSite: 3, LastSurface: "buccal", LastPD: 3}
	for i := 0; i < 3; i++ {
		in, err := c.Interpret(context.Background(), "s", "no wait that was a five", h)
		if err != nil || in.Action != "set_pd" || *in.Value != 5 {
			t.Fatalf("got %+v %v", in, err)
		}
	}
	if n != 1 {
		t.Fatalf("expected 1 upstream call (cache), got %d", n)
	}
}

func TestRateLimitOpensCircuit(t *testing.T) {
	var n int32
	c := mock(t, 429, `"{}"`, &n)
	if _, err := c.Interpret(context.Background(), "s", "a", Hint{}); err != ErrRateLimited {
		t.Fatalf("want ErrRateLimited, got %v", err)
	}
	for i := 0; i < 5; i++ {
		c.Interpret(context.Background(), "s", "b"+string(rune('a'+i)), Hint{})
	}
	if n != 1 {
		t.Fatalf("circuit should block further calls, upstream saw %d", n)
	}
}

func TestBudgetAndValidation(t *testing.T) {
	var n int32
	c := mock(t, 200, `"{\"action\":\"set_pd\",\"value\":99}"`, &n)
	if _, err := c.Interpret(context.Background(), "s", "x", Hint{}); err != ErrBadOutput {
		t.Fatalf("out-of-range value must be rejected, got %v", err)
	}
	c2 := mock(t, 200, `"{\"action\":\"none\"}"`, &n)
	c2.cfg.MinGap = time.Hour
	c2.Interpret(context.Background(), "s", "one", Hint{})
	if _, err := c2.Interpret(context.Background(), "s", "two", Hint{}); err != ErrRateLimited {
		t.Fatalf("per-session gap should throttle, got %v", err)
	}
}
