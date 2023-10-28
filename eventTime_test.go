package fluent

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

func TestEventTimeRoundTripValidBefore2038(t *testing.T) {

	tt := EventTime(time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC))
	buf := &bytes.Buffer{}
	enc := msgpack.NewEncoder(buf)
	err := enc.Encode(&tt)
	if err != nil {
		t.Fatalf("failed to encode Time as msgpack value")
	}
	if buf.Len() != 10 {
		t.Fatalf("expect 10 bytes for serialized Time, got: %d", buf.Len())
	}

	dec := msgpack.NewDecoder(buf)
	tt2 := EventTime{}
	err = dec.Decode(&tt2)
	if err != nil {
		t.Fatalf("failed to decode Time msgpack value: %v", err)
	}

	// Time does not encode timezone
	tt2 = EventTime(time.Time(tt2))

	if !reflect.DeepEqual(tt, tt2) {
		t.Fatalf("orig: %+v, deserialized: %+v", tt, tt2)
	}
}
