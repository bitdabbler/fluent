package fluent

// eventMode defines the Fluent protocol event/carrier mode.
//   - Message mode (included in initial implementation)
//   - Forward mode
//   - PackedForward mode
//   - CompressedPackedForward mode
//     ref: https://github.com/fluent/fluentd/wiki/Forward-Protocol-Specification-v1
type eventMode int

const (
	// MessageMode indicates the Fluent Message mode.
	MessageMode = iota

	// ForwardMode indicates the Fluent Forward mode.
	ForwardMode

	// PackedForwardMode indicates the Fluent PackedForward mode.
	PackedForwardMode

	// CompressedForwardMode indicates the Fluent CompressedPackedForward mode.
	CompressedPackedForwardMode
)

// EncoderOptions are used to customize the Encoders and the Encoder pool.
//
// NB: The struct pointer options approach is used to be consistent with the
// options used for the Handler, which uses the struct pointer approach to be
// consistent with the `HandlerOptions` used by log/slog.
type EncoderOptions struct {
	// Mode is the Fluent event mode (also referred to as the carrier mode in
	// the protocol spec) that applies to all of the logs serialized using
	// Encoders from one shared EncoderPool.
	Mode eventMode

	//NewBufferCap sets the capacity, in bytes, for newly created Encoder
	//buffers. The minimum value is 64 bytes. The default is 1KiB (1<<10).
	NewBufferCap int

	// MaxBufferCap sets the maximum buffer capacity , in bytes, beyond which an
	// Encoder will not be returned to the shared Encoder pool, to prevent rare,
	// unusually large buffers from staying resident in memory. The minimum
	// value is the `newBufferCap`. The default is 8KiB (1<<13).
	MaxBufferCap int

	// WithCoarseTimestamps controls whether the log time is serialized as Unix
	// epoch, which is useful for pre-2016 legacy systems that do not support
	// sub-second precision. The default is false, so timestamps are serialized
	// as Fluent `EventTime` values.
	UseCoarseTimestamps bool

	// RequestACKs controls whether the logs include a ("chunk") request for
	// the Fluent server to send back explicit ACKs.
	RequestACKs bool
}

const (
	minBufferCap        = 64
	defaultNewBufferCap = 1024
	defaultMaxBufferCap = 8192
)

// DefaultEncoderOptions returns *DefaultEncoderOptions with all default values.
func DefaultEncoderOptions() *EncoderOptions {
	return &EncoderOptions{
		NewBufferCap: defaultNewBufferCap,
		MaxBufferCap: defaultMaxBufferCap,
	}
}

// resolve ensures that all options have valid values.
func (o *EncoderOptions) resolve() {
	o.NewBufferCap = max(o.NewBufferCap, minBufferCap)
	o.MaxBufferCap = max(o.NewBufferCap, o.MaxBufferCap)
}
