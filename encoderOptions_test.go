package fluent

import (
	"testing"
)

func TestWitheventMode(t *testing.T) {
	p, err := NewEncoderPool(testTag, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Mode != MessageMode {
		t.Fatal("expected default for `Mode` to be eventMode")
	}
	p, err = NewEncoderPool(testTag, &EncoderOptions{Mode: ForwardMode})
	if err != nil {
		t.Fatal(err)
	}
	if p.Mode != ForwardMode {
		t.Fatal("expected `Mode` to be FowardMode")
	}
}

func TestWithNewBufferCap(t *testing.T) {
	p, err := NewEncoderPool(testTag, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.NewBufferCap != defaultNewBufferCap {
		t.Fatalf("expected newBufferCap default to be: %d, got: %d", defaultNewBufferCap, p.NewBufferCap)
	}

	cap := 16 << 10
	p, err = NewEncoderPool(testTag, &EncoderOptions{NewBufferCap: cap})
	if err != nil {
		t.Fatal(err)
	}
	if p.NewBufferCap != cap {
		t.Fatalf("expected NewBufferCap to be: %d, got: %d`", cap, p.NewBufferCap)
	}
}

func TestWithMaxBufferCap(t *testing.T) {
	p, err := NewEncoderPool(testTag, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.MaxBufferCap != defaultMaxBufferCap {
		t.Fatalf("expected MaxBufferCap default to be: %d, got: %d", defaultMaxBufferCap, p.MaxBufferCap)
	}

	cap := 32 << 10
	p, err = NewEncoderPool(testTag, &EncoderOptions{MaxBufferCap: cap})
	if err != nil {
		t.Fatal(err)
	}
	if p.MaxBufferCap != cap {
		t.Fatalf("expected MaxBufferCap to be: %d, got: %d`", cap, p.MaxBufferCap)
	}
}

func TestWithCoarseTimestamps(t *testing.T) {
	p, err := NewEncoderPool(testTag, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.UseCoarseTimestamps {
		t.Fatal("expected default for `UseCoarseTimestamps` to be false")
	}

	p, err = NewEncoderPool(testTag, &EncoderOptions{UseCoarseTimestamps: true})
	if err != nil {
		t.Fatal(err)
	}
	if !p.UseCoarseTimestamps {
		t.Fatal("expected`UseCoarseTimestamps` to be true")
	}

}

func TestWithRequestACK(t *testing.T) {
	p, err := NewEncoderPool(testTag, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.RequestACKs {
		t.Fatal("expected default for `RequestACKs` to be false")
	}

	p, err = NewEncoderPool(testTag, &EncoderOptions{RequestACKs: true})
	if err != nil {
		t.Fatal(err)
	}
	if !p.RequestACKs {
		t.Fatal("expected`requestACK` to be true")
	}

}
