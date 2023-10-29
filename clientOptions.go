package fluent

import "time"

// ClientOptions are used to customize the Fluent Client.
//
// # Invalid options are coerced
//
// NB: The struct pointer options approach is used to be consistent with the
// options used for the Handler, which uses the struct pointer approach to be
// consistent with the `HandlerOptions` used by log/slog.
type ClientOptions struct {
	// Port of the Fluent server. The default is 24224.
	Port int

	// Network protocol used to communicate with the server. Fluent protocol
	// says "protocol [enum: tcp/udp/tls]". The default is "tcp".
	//   ref: https://docs.fluent.org/configuration/transport-section
	Network string

	// InsecureSkipVerify controls whether a client verifies the server's
	// certificate chain and host name when using TLS.
	InsecureSkipVerify bool

	// DialTimeout sets the timeout for dialing the server. The default is 30s.
	DialTimeout time.Duration

	// SkipEagerDial enables returning clients that dial the server lazily.
	SkipEagerDial bool

	// MaxEagerDialTries limits the maximum number of times client workers will
	// try to connect to establish an initial the server before the Client is
	// returned from the constructor. This is not used if the `SkipEagerDial` is
	// true, or for (re)connections that occur after the constructor returns. If
	// the value is < 0, the constructor will not return until connections are
	// successfully established. The default is 10.
	MaxEagerDialTries int

	// Concurrency controls the number of workers the Client will spin up. Each
	// worker will independently pull messages from the write queue, and send
	// messages to the server over its own connection. The default is 1.
	Concurrency int

	// QueueDepth sets the maximum number of write requests that can be buffered
	// before writing to the server is blocked. If blocked and dropIfQueueIsFull
	// is true, load shedding will occur, with later writes discarded until
	// buffer space increases. The default depth is 0 (synchronous writes).
	QueueDepth int

	// DropIfQueueFull controls how write requests are handled when the
	// writeQueue is full. The default is to block the log handler until the
	// queue channel can receive the log message. With this option enabled,
	// overflow requests will get dropped to the floor. This enables a tradeoff
	// between log completeness and system performance predictability.
	DropIfQueueFull bool

	// WriteTimeout controls the timeout for each Write to the server. If
	// WriteTimeout < 0, then no timeout will be set. The default is 10 seconds.
	WriteTimeout time.Duration

	// MaxWriteTries controls the number of times the net.Conn will try to send
	// a message before inferring a broken pipe, tearing down the connection,
	// and establishing a new one. This must be > 0. The default is 3.
	MaxWriteTries int

	// Verbose controls whether debug logs are written to the internal logger.
	Verbose bool
}

const (
	defaultPort           = 24224
	defaultNetwork        = "tcp"
	defaultDialTimeout    = time.Second * 30
	defaultEagerDialTries = 10
	defaultConcurrency    = 1
	defaultWriteTimeout   = time.Second * 10
	defaultWriteTries     = 3
)

// DefaultClientOptions returns *ClientOptions with all default values.
func DefaultClientOptions() *ClientOptions {
	return &ClientOptions{
		Port:              defaultPort,
		Network:           defaultNetwork,
		DialTimeout:       defaultDialTimeout,
		MaxEagerDialTries: defaultEagerDialTries,
		Concurrency:       defaultConcurrency,
		WriteTimeout:      defaultWriteTimeout,
		MaxWriteTries:     defaultWriteTries,
	}
}

// resolve ensures that all options have valid values.
func (o *ClientOptions) resolve() {
	tell := func(field string, from, to any) {
		InternalLogger().Printf("%s coerced to valid value: %v -> %v", field, from, to)
	}

	// constrain to valid range
	if o.Port < 1024 || o.Port > 65535 {
		tell("Port", o.Port, defaultPort)
		o.Port = defaultPort
	}

	// only [tcp|tls|udp], per Fluent spec
	if o.Network != "tcp" && o.Network != "tls" && o.Network != "udp" {
		tell("Network", o.Network, defaultNetwork)
		o.Network = defaultNetwork
	}

	// must be positive
	if o.DialTimeout < 1 {
		tell("DialTimeout", o.DialTimeout, defaultDialTimeout)
		o.DialTimeout = defaultDialTimeout
	}

	// can be negative (infinity) or positive, but not 0
	if o.MaxEagerDialTries == 0 {
		tell("MaxEagerDialTries", o.MaxEagerDialTries, defaultEagerDialTries)
		o.MaxEagerDialTries = defaultEagerDialTries
	}

	// must have at least one worker
	if o.Concurrency < 1 {
		tell("Concurrency", o.Concurrency, defaultConcurrency)
		o.Concurrency = defaultConcurrency
	}

	// can be negative (infinity) or positive, but not 0
	if o.WriteTimeout == 0 {
		tell("WriteTimeout", o.WriteTimeout, defaultWriteTimeout)
		o.WriteTimeout = defaultWriteTimeout
	}

	// must be positive
	if o.MaxWriteTries < 1 {
		tell("MaxWriteTries", o.MaxWriteTries, defaultWriteTries)
		o.MaxWriteTries = defaultWriteTries
	}

}
