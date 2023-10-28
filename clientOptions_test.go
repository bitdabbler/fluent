package fluent

import (
	"testing"
	"time"
)

func TestClientOptions_resolvedPort(t *testing.T) {

	tests := []struct {
		name   string
		input  int
		expect int
	}{
		{"valid custom port unchanged", 20_000, 20_000},
		{"low port coerced to default", 0, defaultPort},
		{"high port coerced to default", 100_000, defaultPort},
	}
	for i := 0; i < len(tests); i++ {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			opts := &ClientOptions{Port: tt.input}
			opts.resolve()
			if opts.Port != tt.expect {
				t.Errorf("failed: %s, expected: %d, got: %d", tt.name, tt.expect, opts.Port)
			}
		})
	}
}

func TestClientOptions_resolvedNetwork(t *testing.T) {

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"valid network unchanged", "tls", "tls"},
		{"empty network coerced to default", "", defaultNetwork},
	}
	for i := 0; i < len(tests); i++ {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			opts := &ClientOptions{Network: tt.input}
			opts.resolve()
			if opts.Network != tt.expect {
				t.Errorf("failed: %s, expected: %s, got: %s", tt.name, tt.expect, opts.Network)
			}
		})
	}
}

func TestClientOptions_resolvedDialTimeout(t *testing.T) {
	tests := []struct {
		name   string
		input  time.Duration
		expect time.Duration
	}{
		{"valid (positive) DialTimeout unchanged", time.Minute, time.Minute},
		{"0 duration gets coerced to the default", 0, defaultDialTimeout},
		{"negative duration gets coerced to the default", time.Second * -1, defaultDialTimeout},
	}
	for i := 0; i < len(tests); i++ {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			opts := &ClientOptions{DialTimeout: tt.input}
			opts.resolve()
			if opts.DialTimeout != tt.expect {
				t.Errorf("failed: %s, expected: %s, got: %s", tt.name, tt.expect, opts.DialTimeout)
			}
		})
	}
}

func TestClientOptions_resolvedEagerDialTries(t *testing.T) {
	tests := []struct {
		name   string
		input  int
		expect int
	}{
		{"positive MaxEagerDialTries unchanged", 10, 10},
		{"negative MaxEagerDialTries unchanged", -1, -1},
		{"0 eager dial tries gets coerced to the default", 0, defaultEagerDialTries},
	}
	for i := 0; i < len(tests); i++ {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			opts := &ClientOptions{MaxEagerDialTries: tt.input}
			opts.resolve()
			if opts.MaxEagerDialTries != tt.expect {
				t.Errorf("failed: %s, expected: %d, got: %d", tt.name, tt.expect, opts.MaxEagerDialTries)
			}
		})
	}
}

func TestClientOptions_resolvedConcurrency(t *testing.T) {
	tests := []struct {
		name   string
		input  int
		expect int
	}{
		{"valid (positive) concurreny level unchanged", 5, 5},
		{"concurrency of 0 gets cooerced to default", 0, defaultConcurrency},
		{"negative concurrency gets cooerced to default", -1, defaultConcurrency},
	}
	for i := 0; i < len(tests); i++ {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			opts := &ClientOptions{Concurrency: tt.input}
			opts.resolve()
			if opts.Concurrency != tt.expect {
				t.Errorf("failed: %s, expected: %d, got: %d", tt.name, tt.expect, opts.Concurrency)
			}
		})
	}
}

// WriteTimeout must be negative or positive, but not 0 (check the code too)
func TestClientOptions_resolveWriteTimeout(t *testing.T) {
	tests := []struct {
		name   string
		input  time.Duration
		expect time.Duration
	}{
		{"valid (positive) DialTimeout unchanged", time.Minute, time.Minute},
		{"negative duration gets is unchanged", time.Minute, time.Minute},
		{"0 duration gets coerced to the default", 0, defaultWriteTimeout},
	}
	for i := 0; i < len(tests); i++ {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			opts := &ClientOptions{WriteTimeout: tt.input}
			opts.resolve()
			if opts.WriteTimeout != tt.expect {
				t.Errorf("failed: %s, expected: %s, got: %s", tt.name, tt.expect, opts.WriteTimeout)
			}
		})
	}
}

func TestClientOptions_resolvedMaxWriteTries(t *testing.T) {
	tests := []struct {
		name   string
		input  int
		expect int
	}{
		{"positive MaxWriteTries unchanged", 10, 10},
		{"negative MaxWriteTries unchanged", -1, defaultWriteTries},
		{"0 MaxWriteTries gets coerced to the default", 0, defaultWriteTries},
	}
	for i := 0; i < len(tests); i++ {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			opts := &ClientOptions{MaxWriteTries: tt.input}
			opts.resolve()
			if opts.MaxWriteTries != tt.expect {
				t.Errorf("failed: %s, expected: %d, got: %d", tt.name, tt.expect, opts.MaxWriteTries)
			}
		})
	}
}
