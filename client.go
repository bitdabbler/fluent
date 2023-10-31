package fluent

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/bitdabbler/backoff"
)

type worker struct {
	*ClientOptions
	id     int
	conn   net.Conn
	addr   string
	wg     *sync.WaitGroup
	sendCh chan *Encoder
}

// Client represents a Fluent client. If using multiple concurrent client
// workers, then the Client could considered a Fluent client pool, as each
// worker maintains an independent connection to the server.
type Client struct {
	opts    *ClientOptions
	host    string
	workers []*worker
	wg      *sync.WaitGroup
	sendCh  chan *Encoder
}

// New creates a new Fluent client and connects to the Fluent server
// immediately, returning an error if it is unable to establish the connection.
func NewClient(host string, opts *ClientOptions) (*Client, error) {
	return NewClientContext(context.Background(), host, opts)
}

// NewClientContext creates a new Fluent client and connects to the Fluent
// server immediately, returning an error if it is unable to establish the
// initial connections. The Context is passed to `Connect()` can be used to
// cancel the `Connect` operation, or set a global deadline for connecting.
func NewClientContext(ctx context.Context, host string, opts *ClientOptions) (*Client, error) {

	c, err := newClient(host, opts)
	if err != nil {
		return nil, err
	}

	if opts.SkipEagerDial {
		return c, nil
	}

	// eagerly establish server connections from each worker
	for i := 0; i < opts.Concurrency; i++ {
		err = errors.Join(err, c.workers[i].tryConnect(ctx, opts.MaxEagerDialTries))
		if err != nil {
			// will drop the client, so eagerly close open conns
			for j := 0; j < i; j++ {
				c.workers[j].conn.Close()
			}
			return nil, err
		}
	}

	return c, nil
}

func newClient(host string, opts *ClientOptions) (*Client, error) {

	if len(host) == 0 {
		return nil, errors.New("valid host required")
	}

	if opts == nil {
		opts = DefaultClientOptions()
	} else {
		opts.resolve()
	}

	c := &Client{
		opts:    opts,
		host:    host,
		workers: make([]*worker, opts.Concurrency),
		wg:      &sync.WaitGroup{},
		sendCh:  make(chan *Encoder, opts.QueueDepth),
	}

	c.debug("starting Client with the resolved ClientOptions: %+v", c.opts)

	// compose addr to format used by dialers
	addr := fmt.Sprintf("%s:%d", host, opts.Port)

	// add workers and track concurrency
	c.wg.Add(opts.Concurrency)
	for i := 0; i < c.opts.Concurrency; i++ {
		c.workers[i] = &worker{
			ClientOptions: opts,
			id:            i + 1,
			addr:          addr,
			wg:            c.wg,
			sendCh:        c.sendCh,
		}
		go c.workers[i].run()
	}

	return c, nil
}

func (w *worker) tryConnect(ctx context.Context, maxAttempts int) error {
	w.debug("attempting to connect to Fluent server\n")

	b, err := backoff.New(
		backoff.WithInitialDelay(0),
		backoff.WithExponentialLimit(time.Second*20),
	)
	if err != nil {
		return err
	}

	i := 0
	for {
		i++
		err = w.connect(ctx)
		if err == nil {
			w.debug("successfully connected to Fluent server\n")
			return nil
		}

		w.debug("failed to connect to Fluent server on attempt %d: %v\n", i, err)

		if maxAttempts > 0 && i > maxAttempts {
			break
		}

		b.Sleep()
	}

	return fmt.Errorf("failed connect to Fluent server; maxAttempts reached: %d: %w", maxAttempts, err)
}

func (w *worker) connect(ctx context.Context) error {

	var d net.Dialer
	ctx, cancel := context.WithTimeout(ctx, w.DialTimeout)
	defer cancel()

	w.debug("dialing Fluent server at %s over %s\n", w.addr, w.Network)

	switch w.Network {
	case "tcp":

		conn, err := d.DialContext(ctx, "tcp", w.addr)
		if err != nil {
			return fmt.Errorf("failed to dial server: addr: %s: network:  %s: %w", w.addr, w.Network, err)
		}
		w.conn = conn

	case "tls":
		tlsDialer := tls.Dialer{
			NetDialer: &d,
			Config:    &tls.Config{InsecureSkipVerify: w.InsecureSkipVerify},
		}
		conn, err := tlsDialer.DialContext(ctx, "tcp", w.addr)
		if err != nil {
			return fmt.Errorf("failed to dial Fluent server at %s over protocol %s: %w", w.addr, w.Network, err)
		}
		w.conn = conn
	case "udp":
		conn, err := d.DialContext(ctx, "udp", w.addr)
		if err != nil {
			return fmt.Errorf("failed to dial Fluent server at %s over protocol %s: %w", w.addr, w.Network, err)
		}
		w.conn = conn
	default:
		return fmt.Errorf("unsupported Fluent client transport protocol: %s", w.Network)
	}

	return nil
}

func (w *worker) run() {

	// loop until the fan-in messageCh closes
	for enc := range w.sendCh {

	reconnectloop:
		for {
			// nil when (a) using lazy conns, (b) after broken pipe tear down
			if w.conn == nil {
				w.debug("reconnectloop: not connected to server\n")

				// ignoring this error because with 0 (infinite) retries, this
				// won't return until the conn is established and err == nil
				w.tryConnect(context.Background(), 0)
			}

			// write to the server; retry if recoverable
			for i := 0; i < w.MaxWriteTries; i++ {
				if w.WriteTimeout > 0 {
					w.conn.SetWriteDeadline(time.Now().Add(w.WriteTimeout))
				}

				_, err := w.conn.Write(enc.Bytes())
				if err == nil {
					break reconnectloop
				}

				// only consider timeouts potentially recoverable
				if ne, ok := err.(net.Error); !(ok && ne.Timeout()) {
					w.reportError("failed to Write message: unrecoverable error: %v\n", err)
					break
				}

				w.debug("failed to Write message: attempt %d: recoverable error: %v\n", i, err)
			}

			// either non-recoverable error or we exhausted maxWriteTries
			w.debug("broken pipe detected; tearing down connection")
			err := w.conn.Close()
			if err != nil {
				w.reportError("error closing broken connection: %v", err)
			}
			w.conn = nil
		}

		// successfully wrote the bytes out to the server
		enc.Free()
	}

	w.debug("closing net.Conn and returning from worker goroutine")

	// if using lazy connections and the channel is closed before any write
	// requests are pushed into it, then the conn could still be nil
	if w.conn != nil {
		w.conn.Close()
	}

	w.wg.Done()
}

// Send places the log payload Encoder into the write queue.
//
// This operation is sync/blocking operation when:
//   - the queueDepth is 0, or
//   - the queue is full and dropIfQueueIsFull is false
//
// This operation is async/non-blocking when:
//   - queueDepth > 0, and
//   - the queue is not full, or dropIfQueueIsFull is true
//
// The payload should NOT include the Fluent `option` field, which the Client
// is responsible for adding if necessary.
func (c *Client) Send(enc *Encoder) {
	if c.opts.DropIfQueueFull {
		select {
		case c.sendCh <- enc:
		default:
			c.debug("full buffer: dropping write request: queue depth: %d", c.opts.QueueDepth)
		}
	} else {
		// otherwise block if the queue is full
		c.sendCh <- enc
	}
}

// Shutdown is used to support graceful shutdown. It closes the write queue
// channel, so any further calls to Send* methods will panic. Shutdown blocks
// until the write queue is fully drained and all worker goroutines have
// stopped, or the context expires, whichever occurs first.
func (c *Client) Shutdown(ctx context.Context) error {
	close(c.sendCh)
	c.debug("message send queue closed; writing out previously enqueued messages")

	doneCh := make(chan error, 1)
	go func() {
		c.wg.Wait()
		close(doneCh)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneCh:
		c.debug("message send queue successfully drained")
		return nil
	}
}

// internal logging helpers:
func (c *Client) debug(format string, args ...any) {
	if !c.opts.Verbose {
		return
	}
	InternalLogger().Printf(format, args...)
}

func (w *worker) debug(format string, args ...any) {
	if !w.Verbose {
		return
	}
	args = append([]any{w.id}, args...)
	InternalLogger().Printf("worker %d: "+format, args...)
}

func (w *worker) reportError(format string, args ...any) {
	args = append([]any{w.id}, args...)
	InternalLogger().Printf("worker %d: "+format, args...)
}
