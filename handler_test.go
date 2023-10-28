package fluent

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"time"
)

func TestSlogger_SlogtestTestClient(t *testing.T) {

	c := newTestClient()
	p, err := NewEncoderPool(testTag, nil)
	if err != nil {
		t.Fatalf("failed to get EncoderPool: %v", err)
	}
	h := NewHandlerCustom(c, p, &HandlerOptions{Verbose: true})

	results := func() []map[string]any {
		return c.recordsWithTimeInlined()
	}

	if err := TestHandler(h, results); err != nil {
		t.Fatalf("failed TestHandler: %v", err)
	}
}

func TestSlogger_HandlesContextValues(t *testing.T) {
	host := "127.0.0.1"
	groupKey := "req"

	ts, err := newTestServer(nil)
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}

	c, err := NewClient(host, &ClientOptions{
		Port:    ts.port,
		Verbose: true,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	p, err := NewEncoderPool(testTag, nil)
	if err != nil {
		t.Fatalf("failed to get EncoderPool: %v", err)
	}
	h := NewHandlerCustom(c, p, nil)
	l := slog.New(h)

	m := TestMessage{
		Tag:  testTag,
		Time: time.Now(),
		Record: map[string]any{
			"msg":    "this is a test message",
			"level":  "INFO",
			"key-01": "value-01",
			"req":    map[string]any{"method": "GET"},
		},
	}

	ctx := context.WithValue(context.Background(), ContextKey, slog.Group(groupKey, m.Record["req"]))
	l.InfoContext(ctx, "message-1", "key-1", "value-1")

	received := <-ts.messageCh
	if ctxVal, ok := received.Record[groupKey]; !ok {
		t.Fatalf("missing slog.Attr passed via context: '%s'", groupKey)
	} else if reflect.DeepEqual(ctxVal, m.Record[groupKey]) {
		t.Fatalf("expected: %+v, got: %+v", m.Record[groupKey], ctxVal)
	}
}
