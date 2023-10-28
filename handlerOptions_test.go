package fluent

import (
	"log"
	"log/slog"
	"testing"
)

func TestWithSourceInfo(t *testing.T) {
	c := newTestClient()
	p, err := NewEncoderPool(testTag, nil)
	if err != nil {
		t.Fatalf("failed to get EncoderPool: %v", err)
	}
	h := NewHandlerCustom(c, p, nil)

	// check config
	if h.AddSource {
		t.Fatal("expected default for `AddSource` to be false")
	}

	// check results
	l := slog.New(h)
	l.Info("test-msg", "k", "v")
	if _, ok := c.logs[len(c.logs)-1].Record[slog.SourceKey]; ok {
		t.Fatal("expected default NOT to include source info")
	}

	// new handler with option enabled
	h = NewHandlerCustom(c, p, &HandlerOptions{AddSource: true})

	// check config
	if !h.AddSource {
		t.Fatal("expected`AddSource` to be true")
	}

	// check results
	l = slog.New(h)
	l.Info("test-msg", "k", "v")
	if src, ok := c.logs[len(c.logs)-1].Record[slog.SourceKey]; !ok {
		t.Fatal("missing source info")
	} else {
		log.Println(src)
	}
}

func TestHandler_LogLevelOption(t *testing.T) {
	c := newTestClient()
	p, err := NewEncoderPool(testTag, nil)
	if err != nil {
		t.Fatalf("failed to get EncoderPool: %v", err)
	}
	h := NewHandlerCustom(c, p, nil)

	// check config
	if h.AddSource {
		t.Fatal("expected default for `AddSource` to be false")
	}

	// check results
	l := slog.New(h)
	l.Info("test-msg", "k", "v")
	if _, ok := c.logs[len(c.logs)-1].Record[slog.SourceKey]; ok {
		t.Fatal("expected default NOT to include source info")
	}

	// new handler with option enabled
	h = NewHandlerCustom(c, p, &HandlerOptions{AddSource: true})

	// check config
	if !h.AddSource {
		t.Fatal("expected`AddSource` to be true")
	}

	// check results
	l = slog.New(h)
	l.Info("test-msg", "k", "v")
	if src, ok := c.logs[len(c.logs)-1].Record[slog.SourceKey]; !ok {
		t.Fatal("missing source info")
	} else {
		log.Println(src)
	}
}

func TestHandler_TimeFormatOption(t *testing.T) {
	c := newTestClient()
	p, err := NewEncoderPool(testTag, nil)
	if err != nil {
		t.Fatalf("failed to get EncoderPool: %v", err)
	}
	h := NewHandlerCustom(c, p, nil)

	// check config
	if h.AddSource {
		t.Fatal("expected default for `AddSource` to be false")
	}

	// check results
	l := slog.New(h)
	l.Info("test-msg", "k", "v")
	if _, ok := c.logs[len(c.logs)-1].Record[slog.SourceKey]; ok {
		t.Fatal("expected default NOT to include source info")
	}

	// new handler with option enabled
	h = NewHandlerCustom(c, p, &HandlerOptions{AddSource: true})

	// check config
	if !h.AddSource {
		t.Fatal("expected`AddSource` to be true")
	}

	// check results
	l = slog.New(h)
	l.Info("test-msg", "k", "v")
	if src, ok := c.logs[len(c.logs)-1].Record[slog.SourceKey]; !ok {
		t.Fatal("missing source info")
	} else {
		log.Println(src)
	}
}
