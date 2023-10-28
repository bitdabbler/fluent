package fluent

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestClient_Send(t *testing.T) {
	verbose := false

	ts, err := newTestServer(&testServerOptions{verbose: verbose})
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer ts.Shutdown()

	c, err := NewClient(testHost, &ClientOptions{
		Port:              ts.port,
		MaxEagerDialTries: 1,
		DialTimeout:       time.Second,
		Verbose:           verbose,
	})
	if err != nil {
		t.Fatalf("failed to get NewClient: %v", err)
	}

	m := &TestMessage{
		Tag:  "test-tag",
		Time: time.Now().UTC(),
		Record: map[string]any{
			"key-01": "value-01",
			"key-02": "value-02",
		},
	}

	// encode directly
	p, err := NewEncoderPool(m.Tag, nil)
	if err != nil {
		t.Fatalf("failed to get EncoderPool: %v", err)
	}
	enc := p.Get()
	err = enc.EncodeEventTime(m.Time)
	err = errors.Join(err, enc.EncodeMap(m.Record))
	if err != nil {
		t.Fatalf("failed to encode the test message: %v\n", err)
	}

	c.Send(enc)

	timeout, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case <-timeout.Done():
		t.Fatalf("test message m was not received in time")
	case m2 := <-ts.messageCh:
		if !reflect.DeepEqual(m, m2) {
			t.Fatalf("\nexpected: %+v\nreceived: %+v", m, m2)
		}
	}
}
