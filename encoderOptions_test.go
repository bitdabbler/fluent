package fluent

import (
	"testing"
)

func TestWitheventMode(t *testing.T) {
	p := NewEncoderPool(testTag, nil)
	if p.Mode != MessageMode {
		t.Fatal("expected default for `Mode` to be eventMode")
	}
	p = NewEncoderPool(testTag, &EncoderOptions{Mode: ForwardMode})
	if p.Mode != ForwardMode {
		t.Fatal("expected `Mode` to be FowardMode")
	}
}

func TestWithNewBufferCap(t *testing.T) {
	p := NewEncoderPool(testTag, nil)
	if p.NewBufferCap != defaultNewBufferCap {
		t.Fatalf("expected newBufferCap default to be: %d, got: %d", defaultNewBufferCap, p.NewBufferCap)
	}

	cap := 16 << 10
	p = NewEncoderPool(testTag, &EncoderOptions{NewBufferCap: cap})
	if p.NewBufferCap != cap {
		t.Fatalf("expected NewBufferCap to be: %d, got: %d`", cap, p.NewBufferCap)
	}
}

func TestWithMaxBufferCap(t *testing.T) {
	p := NewEncoderPool(testTag, nil)
	if p.MaxBufferCap != defaultMaxBufferCap {
		t.Fatalf("expected MaxBufferCap default to be: %d, got: %d", defaultMaxBufferCap, p.MaxBufferCap)
	}

	cap := 32 << 10
	p = NewEncoderPool(testTag, &EncoderOptions{MaxBufferCap: cap})
	if p.MaxBufferCap != cap {
		t.Fatalf("expected MaxBufferCap to be: %d, got: %d`", cap, p.MaxBufferCap)
	}
}

func TestWithCoarseTimestamps(t *testing.T) {
	p := NewEncoderPool(testTag, nil)
	if p.UseCoarseTimestamps {
		t.Fatal("expected default for `UseCoarseTimestamps` to be false")
	}

	p = NewEncoderPool(testTag, &EncoderOptions{UseCoarseTimestamps: true})
	if !p.UseCoarseTimestamps {
		t.Fatal("expected`UseCoarseTimestamps` to be true")
	}

}

func TestWithRequestACK(t *testing.T) {
	p := NewEncoderPool(testTag, nil)
	if p.RequestACKs {
		t.Fatal("expected default for `RequestACKs` to be false")
	}

	p = NewEncoderPool(testTag, &EncoderOptions{RequestACKs: true})
	if !p.RequestACKs {
		t.Fatal("expected`requestACK` to be true")
	}

}
