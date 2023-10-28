# fluent

A full Fluent logging stack in Go, including
 
- `fluent.Handler` - serializes structured logs (implements `slog.Handler`)
- `fluent.Client` - manages connections and writing to the Fluent server
- `fluent.Encoder` - provides a common encoder/buffer, bridging the `Handler` and `Client` 

## Purpose

Why write a yet another Fluent client and yet another structured log handler? In short, efficiency.

Let's focus on  the Fluent `Message` event mode, with the msgpack form: `[tag, time, record, option]`, where `record` is equivalent to the Go type `map[string]any`.

ref: https://github.com/fluent/fluent/wiki/Forward-Protocol-Specification-v1#message-modes

Consider an API such as:

```go
// NOT this libary
client.Send(tag string, timestamp time.Time, record map[string]any)
```

This hypothetical API (similar to those found in existing libraries) demonstrates the inefficiencies we seek to avoid. First, the log attributes (key-value pairs) had to be collected into the `map[string]any` for the record field. Second, all of the fields for the `Message` have to be copied across one extra function boundary. Additionally, those fields are often then collected into an object that represents one `Message`.

For optimal efficiency, the `fluent.Client` sends directly from the buffers used by the `fluent.Handler`. This allows log handlers to serialize log values without first marshaling them into intermediate objects, avoiding redundant serialization steps and excess copying. 

Efficiency optimizations:

- the `Handler` and the `Client` directly use the same encoders/buffers 
- comprehensive resource pooling to minimize heap allocations
- log preludes are encoded only once per pool, and are copied into each
  `Encoder` only once, no matter how many times the Encoder is used
- shared log attributes (`WithAttrs`) are encoded only once, no
  matter how many times they are used by the Handler
- where map values have a length that can change, we
  overallocate a single byte for the msgpack map header, so that we perform
  neither look-ahead nor extra copying when the length changes (an example of
  this occurs when the key of a child group `Attr` is an empty string, causing
  its own `Attr`s to get serialized into the parent's scope)

## Basic Usage

```go
import (
    // ...
    github.com/bitdabbler/fluent
)
```

```go
h, err := fluent.NewHandler(fluentHost, fluentTag, nil)
if err != nil {
    log.Fatal(err)
}
l := slog.New(h)

// use locally
l.Info("the message", slog.Int("key", 42))

// or set it as the slog package-level logger
slog.SetDefault(l)

// and then use it globally
slog.Info("the message", slog.Int("key", 42))
```

## Details

In the above example, we let the `Handler` deal with setting up the `Client` and the `EncoderPool`. 

For the `Client`, it sets the level of concurrency to 2, and the send queue depth to 16, making writes to the server asynchronous. For the `EncoderPool`, it uses only the default `EncoderOptions`. However, you can use an alternative constructor to fully customize everything.

```go
// customize the Client
c, err := fluent.NewClient(fluentHost, &fluent.ClientOptions{
    Port: fluentPort,
    DialTimeout: time.Seconds * 5,
    SkipEagerDial: true,
})
if err != nil {
  log.Fatal(err)
}

// customize the EncoderPool
p, err := fluent.NewEncoderPool(fluentTag, &fluent.EncoderOptions{
    UseCoarseTimestamps: true,
})
if err != nil {
  log.Fatal(err)
}

// customize the Handler
h := fluent.NewHandlerCustom(c, p, &fluent.HandlerOptions{
    AddSource: true,
    TimeFormat: time.RFC1123Z,
})

l := slog.New(h)
slog.SetDefault(l)
slog.Info("another message", slog.String("path", "/enlightenment"))
```

### fluent.Handler

#### Constructors:

- `NewHandler(host, tag string, opts *HandlerOptions) (*Handler, error)`
- `NewHandlerCustom(client Sink, pool *EncoderPool, opts *HandlerOptions) *Handler`

#### Configuration options

| Option       | Type         | Default          |
| ------------ | ------------ | ---------------- |
| `AddSource`  | bool         | false            |
| `TimeFormat` | string       | time.RFC3339Nano |
| `Level`      | slog.Leveler | slog.LevelInfo   |
| `Verbose`    | bool         | false            |

#### Passing log values through context.Context

A `Handler` can extract a `slog.Attr` from a `context.Context`. You can use a `slog.Group` to add multiple values.

```go
// use the fluent.ContextKey
ctx := context.WithValue(context.Background(), fluent.ContextKey, 
	slog.Group("req",
		slog.String("method", r.Method),
		slog.String("url", r.URL.String()),
	)
)

// log with context, resulting in a payload with the record field:
// {level:INFO,msg:success,req:{method:Get,url:www.example.com}}
slog.InfoContext(ctx, "success") 
```

#### Graceful shutdown

The `Shutdown` method calls `Client.Shutdown()`. That immediately closes the send queue channel, so the caller must guarantee that no more calls to the `Handler` methods will occur. 

`Shutdown` blocks while the send queue is drained and all workers shutdown.

```go
// we: 
//   - are in a higher level graceful shutdown function
//   - used slog.SetDefault(slogger) to ensure it was used globally

// create a new Handler that only logs locally to stdout
l := slog.New(slog.NewJSONHandler(os.Stdout, nil))

// atomically switch over to that logger, so that no subsequent
// logging calls will use the `slogger` instance
slog.SetDefault(l)

// now it is safe to shutdown the Handler instance's Client
//
// this blocks until either
//   (a) the write queue is completely drained, or
//   (b) the timeout expired (no limit with context.Background())
h.Shutdown(timeoutCtx)
```

### fluent.Client

#### Constructors:

- `NewClient(host string, opts *ClientOptions) (*Client, error)`
- `NewClientContext(ctx context.Context, host string, opts *ClientOptions) (*Client, error)`

#### Configuration options

| Option               | Type          | Default                          |
| -------------------- | ------------- | -------------------------------- |
| `Port`               | int           | 24224                            |
| `Network`            | string        | tcp                              |
| `InsecureSkipVerify` | bool          | false                            |
| `DialTimeout`        | time.Duration | 30 seconds                       |
| `SkipEagerDial`      | int           | false (connect eagerly in `New`) |
| `MaxEagerDialTries`  | int           | 10                               |
| `Concurrency`        | int           | 1                                |
| `QueueDepth`         | int           | 0 (writes are synchronous)       |
| `DropIfQueueFull`    | bool          | false (blocks if queue is full)  |
| `WriteTimeout`       | time.Duration | 0 (no timeout)                   |
| `MaxWriteTries`      | int           | 3                                |
| `Verbose`            | bool          | false                            |

#### Concurrency

Use the concurrency settings to enable the Client spin up mutliple workers internally. The workers maintain completely independent connections to the server, for thread safety with minimal locking. The default concurrency level is 1, ensuring that all logs are written out serially.

### fluent.Encoder(Pool)

#### Constructors

- `NewEncoderPool(tag string, opts *EncoderOptions) (*EncoderPool, error)`
- `NewEncoder(bufferCap int) *Encoder`

#### Configuration Options

| Functional option     | ---              | Default     |
| --------------------- | ---------------- | ----------- |
| `Mode`                | fluent.EventMode | MessageMode |
| `NewBufferCap`        | int              | 1KiB        |
| `MaxBufferCap`        | int              | 8KiB        |
| `UseCoarseTimestamps` | bool             | false       |
| `RequestACKs`         | bool             | false       |

## Design Decisions, Tradeoffs, and Current Limitations

Not Implemented (yet):

- Handshake messages
- [Compressed][Packed]Forward event modes (and related options)
- explicit ACKing

The current structures and interfaces were designed with Forward event mode and explicit ACK support in mind, so the path to implement them should be smooth.

### Explicit ACKs

In the [Option](https://github.com/fluent/fluent/wiki/Forward-Protocol-Specification-v1#option), the protocol specification discusses the `chunk` option, stating:

> "chunk: Clients MAY send the chunk option to confirm the server receives event records. The value is a string of Base64 representation of 128 bits unique_id which is an ID of a *set of events*." (emphasis added). 

The `chunk` option is used for explicit ACKing. Whether that is intended to apply to Message event node is ambiguous. It refers to a "set of events", which relates to the other event modes, not the Message event mode, where each message includes only a single event. Additionally, prior to the Option section, the spec repeatedly and exclusively uses "chunk" to refer a binary chunk of a MessagePackEventStream. 

On the other hand, the Message specification includes an optional 4th "option" value in the msgpack array, chunk/ACK support is the only option that appears applicable to Message event mode, and other libraries have included ACK support with this mode.

### JSON serialization

JSON serialization is not implemented, as it is less efficient and offers no functional advantage. The log forwarder and the tools used to review logs are separate concerns. The serialized key-value pairs should appear the same regardless of how they are serialized and transported.

### Attr Rewrite Hook

We do not currently provide an Attr rewriting hook analogous to the `ReplaceAttr` hook provided by the standard library's HandlerOptions, used by the built in TextHandler and JSONHandler. Omitting it a provides only a minimal efficiency gain, and results in the inability to rewrite Attr _keys_ dynamically. 

However, the main use case for Attr rewriting is to redact sensitive values or change the logged _value_, not the key. This functionality is better handled by wrapping the value in a type that implements LogValuer, as seen in the example https://pkg.go.dev/log/slog#example-LogValuer-Secret.

### Protocol/Specification References

- [Fluent Protocol v1](https://github.com/fluent/fluent/wiki/Forward-Protocol-Specification-v1)
- [MsgPack](https://github.com/msgpack/msgpack/blob/master/spec.md)
- [Go's slog.Handler interface](https://github.com/golang/go/blob/master/src/log/slog/handler.go)
