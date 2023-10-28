package fluent

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

// Pool defines a shared *Encoder pool, used to minimize heap allocations.
type EncoderPool struct {
	p sync.Pool
	*EncoderOptions
	preludeLen int
}

// NewEncoderPool creates a shared *Encoder pool that returns Encoders with the
// log prelude, including the outer msgpack array and the tag, pre-encoded.
func NewEncoderPool(tag string, opts *EncoderOptions) *EncoderPool {
	if opts == nil {
		opts = DefaultEncoderOptions()
	} else {
		opts.resolve()
	}

	ep := &EncoderPool{EncoderOptions: opts}

	ep.p = sync.Pool{
		New: func() any {
			enc := NewEncoder(opts.NewBufferCap)
			enc.p = ep

			// encode the prelude
			var arrayLen int
			switch opts.Mode {
			case MessageMode:
				arrayLen = 3
				if opts.RequestACKs {
					arrayLen++
				}
			case ForwardMode:
				arrayLen = 2
				if opts.RequestACKs {
					arrayLen++
				}
			case PackedForwardMode, CompressedPackedForwardMode:
				arrayLen = 3
			}
			if err := enc.EncodeArrayLen(arrayLen); err != nil {
				InternalLogger().Printf("failed to encode NewEncoder array len: %v", err)
			}
			if err := enc.EncodeString(tag); err != nil {
				InternalLogger().Printf("failed to encode NewEncoder tag: %v", err)
			}
			ep.preludeLen = enc.Len()

			return enc
		},
	}

	return ep
}

// Get returns an Encoder with the prelude pre-rendered.
func (p *EncoderPool) Get() *Encoder {
	e := p.p.Get().(*Encoder)
	return e
}

// Put resets an Encoder and returns it to the shared pool.
func (p *EncoderPool) Put(e *Encoder) {

	// drop if the buffer got too large
	if e.Buffer.Cap() > p.MaxBufferCap {
		return
	}

	// reset for the next usage
	e.Buffer.Truncate(p.preludeLen)
	e.Encoder.Reset(e.Buffer)

	// add back to the sync.Pool
	p.p.Put(e)
}

// Encoder provides a mspgack encoder and its underlying bytes.Buffer.
type Encoder struct {
	*bytes.Buffer
	*msgpack.Encoder
	p *EncoderPool
}

// NewEncoder returns a newly allocated Encoder.
func NewEncoder(bufferCap int) *Encoder {
	buf := bytes.NewBuffer(make([]byte, 0, bufferCap))
	return &Encoder{
		Buffer:  buf,
		Encoder: msgpack.NewEncoder(buf),
	}
}

// Free returns the encoder to the shared pool after eagerly resetting it.
func (e *Encoder) Free() {
	e.p.Put(e)
}

// DeepCopy returns a deep copy of the Encoder.
func (e *Encoder) DeepCopy() *Encoder {
	// pooled Encoder with pre-rendered prelude
	if e.p != nil {
		e2 := e.p.Get()
		if e.Cap() > e2.Cap() {
			e2.Grow(e.Cap())
		}

		// TODO: from offset
		e2.Write(e.Bytes()[e.p.preludeLen:])
		return e2
	}

	// raw Encoder with no prelude
	e2 := NewEncoder(e.Cap())
	e2.Write(e.Bytes())
	return e2
}

// EncodeTimestamp is a helper that by default encodes a time value as a custom
// msgpack type defined by Fluent (EventTime). If the Encoder is set to use
// coarse timestamps, then it encodes the time value as 64-bit integer
// representing Unix epoch.
func (e *Encoder) EncodeEventTime(utc time.Time) error {

	// no timezone support in Fluent spec; ensure time is in UTC
	utc = utc.In(time.UTC)

	if e.p.UseCoarseTimestamps {
		if err := e.EncodeInt64(utc.Unix()); err != nil {
			return fmt.Errorf("failed to encode timestamp as int64: %w", err)
		}
		return nil
	}

	t := EventTime(utc)
	if err := e.Encode(&t); err != nil {
		return fmt.Errorf("failed to encode timestamp as EventTime: %w", err)
	}

	return nil
}

// Mode returns the Fluent event mode of the Encoder.
func (e *Encoder) Mode() eventMode { return e.p.Mode }

// UseCoarseTimestamps controls whether legacy (unix epoch) timestamps are used.
func (e *Encoder) UseCoarseTimestamps() bool { return e.p.UseCoarseTimestamps }

// RequestACK controls whether explicit ACKS are requested from the server.
func (e *Encoder) RequestACK() bool { return e.p.RequestACKs }
