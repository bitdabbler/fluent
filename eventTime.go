package fluent

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

// Fluent does not use the predefined (type -1) msgpack Time serialization
// format for sub-second precision, but instead defines a unique serialization
// format, assinging extension type 0.
//
// +-------+----+----+----+----+----+----+----+----+----+
// |     1 |  2 |  3 |  4 |  5 |  6 |  7 |  8 |  9 | 10 |
// +-------+----+----+----+----+----+----+----+----+----+
// |    D7 | 00 | second from epoch |     nanosecond    |
// +-------+----+----+----+----+----+----+----+----+----+
// |fixext8|type| 32bits integer BE | 32bits integer BE |
// +-------+----+----+----+----+----+----+----+----+----+
//
//   ref: https://github.com/fluent/fluent/wiki/Forward-Protocol-Specification-v1#time-ext-format
//

type EventTime time.Time

// compile-time check for msgpack Custom[En|De]coder conformance
var _ msgpack.CustomEncoder = (*EventTime)(nil)
var _ msgpack.CustomDecoder = (*EventTime)(nil)

const (
	TimeExtType = 0
	TimeLen     = 8
)

// EncodeMsgpack serializes *Time values to the custom msgpack format
// defined in the Fluent protocol specification.
func (t *EventTime) EncodeMsgpack(enc *msgpack.Encoder) error {
	w := enc.Writer()
	err := enc.EncodeExtHeader(TimeExtType, TimeLen)
	if err != nil {
		return fmt.Errorf("failed to encode Time header: %w", err)
	}

	// get the time value in UTC
	utc := time.Time(*t).UTC()

	// serialize seconds
	// NB: 64bit -> 32bit => constrained to 1970-2038
	err = binary.Write(w, binary.BigEndian, uint32(utc.Unix()))
	if err != nil {
		return fmt.Errorf("failed to encode seconds component of Time: %w", err)
	}

	// serialize nanoseconds
	err = binary.Write(w, binary.BigEndian, uint32(utc.Nanosecond()))
	if err != nil {
		return fmt.Errorf("failed to encode nanoseconds component of Time: %w", err)
	}

	return nil
}

// DecodeMsgpack deserializes *Time values from the custom msgpack format
// defined in the Fluent protocol specification.
func (t *EventTime) DecodeMsgpack(dec *msgpack.Decoder) error {

	buf := make([]byte, 10)

	// read out the 10 bytes for the serialized EventType
	err := dec.ReadFull(buf)
	if err != nil {
		return fmt.Errorf("failed to decode EventTime: %w", err)
	}

	// validate header
	if buf[0] != 0xD7 {
		return fmt.Errorf("failed to decode EventTime: byte[0] = %X, expected: 0xD7 (fixext8)", buf[0])
	}
	if buf[1] != 0x00 {
		return fmt.Errorf("failed to decoding EvetTime: byte[1] = %X, expected: 0x00 (custom type 0)", buf[1])
	}

	// convert back to a valid time.Time value wrapped as an Time
	secs := int64(binary.BigEndian.Uint32(buf[2:6]))
	nsecs := int64(binary.BigEndian.Uint32(buf[6:]))
	*t = EventTime(time.Unix(secs, nsecs).In(time.UTC))

	return nil
}
