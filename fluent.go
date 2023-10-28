/*
Package fluent provides full Fluent logging stack in Go, including:

  - `fluent.Handler` - serializes structured logs (implements `slog.Handler`)
  - `fluent.Client` - manages connections and writing to the Fluent server
  - `fluent.Encoder` - provides a common encoder/buffer, bridging the `Handler`
    and `Client`

The stack is optimized for efficiency, and the `Handler` and `Client` are
coupled only via their shared use of the `Encoder`.

Examples of efficiency optimizations:

  - shared encoders/buffers
  - comprehensive use of resource pooling to minimize heap allocations
  - log preludes are only encoded once per pool, and only copied into each
    `Encoder` in the pool, no matter how many times the Encoder is used
  - shared log attributes (`WithAttrs`) are encoded only once, no
    matter how many times they are used by the Handler
  - where map values have a length that can change, we
    overallocate a single byte for the msgpack map header, so that we perform
    neither look-ahead nor extra copying when the length changes (an example of
    this occurs when the key of a child group `Attr` is an empty string, causing
    its own `Attr`s to get serialized into the parent's scope)
*/
package fluent
