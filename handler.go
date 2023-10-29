package fluent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"runtime"
	"time"

	"github.com/vmihailenco/msgpack/v5/msgpcode"
)

type ccKey struct{}

// ContextKey is used to extract a log value from context.Context. The value
// must be be `slog.Attr`.
//
//		Example:
//	 	ctx := context.WithValue(ctx, fluentHandler.ContextKey,
//	 		slog.Group("req",
//	 			slog.String("method", r.Method),
//	 			slog.String("url", r.URL.String()),
//	 		)
//	 	)
//
// These attrs are added to the top scope of the Fluent message record.
var ContextKey *ccKey = &ccKey{}

// scope provides the meta data for one scope of logs. WithGroup() is used to
// create new, nested scopes.
type scope struct {
	key    string
	offset int // first byte for this scope in pre-encoded attrs buffer
	nAttrs int
}

// Handler is an adapter that serializes Go structured logs out to Fluent
// msgpack arrays, without first serializing them to intermediate data
// structures, such as map[string]any.
//
//	// Example of basic usage
//	h, err := fluent.NewHandler(fluentHost, fluentTag, nli)
//	if err != nil {
//	   log.Fatalln(err)
//	}
//
//	logger := slog.New(h)
//	slog.SetDefault(logger)
//
//	slog.Info("unrecognized user", "user_id", user_id)
type Handler struct {
	*HandlerOptions
	client  Sink
	logPool *EncoderPool
	preEnc  *Encoder
	scopes  []scope

	// used for padded maplen encoding
	buf3 [3]byte
}

// NewHandler wraps uses an EncoderPool with default options, and a Client that
// uses default options except Concurrency is set to 2 and The QueueDepth is set
// to 16, for asynchronous sending.
//
// For complete control over the `fluent.Client` and the encoding options used
// by the `fluent.Encoder`s, use the `NewHandlerCustom` constructor.
func NewHandler(host, tag string, opts *HandlerOptions) (*Handler, error) {

	// get the encoder pool with default encoding options
	p, err := NewEncoderPool(tag, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create fluent.NewEncoderPool: %w", err)
	}

	// customize the client as noted
	c, err := NewClient(host, &ClientOptions{
		Concurrency: 2,
		QueueDepth:  16,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create fluent.NewClient: %w", err)
	}

	return NewHandlerCustom(c, p, opts), nil
}

// Sink interface defines the Client API.
type Sink interface {
	Send(*Encoder)
	Shutdown(context.Context) error
}

// NewHandlerCustom allows creates a Handler that wrap a Client and an
// EncoderPool that are fully customizable by the caller.
func NewHandlerCustom(client Sink, pool *EncoderPool, opts *HandlerOptions) *Handler {
	if opts == nil {
		opts = DefaultHandlerOptions()
	} else {
		opts.resolve()
	}

	return &Handler{
		HandlerOptions: opts,
		client:         client,
		logPool:        pool,
		scopes:         make([]scope, 1), // 1 for the root scope
		preEnc:         NewEncoder(1024),
	}
}

// Shutdown closes the writeQueue used to connect the logger to the Fluent
// client workers. You MUST NOT call any other logger methods after calling
// Shutdown. This method will block until writeQueue is fully drained.
func (h *Handler) Shutdown(ctx context.Context) error {
	h.debug("shutting down the logging stack")
	return h.client.Shutdown(ctx)
}

// deepcopy creates a copy of the Handler that can be independently modified
// moving forward without impacting the parent handler it derives from; that
// requires deep copies of the log attr tracking properties, including the scope
// definitions, the msgpack encoder, and its underlying bytes buffer
func (h *Handler) deepCopy() *Handler {
	// make a copy of the handler
	h2 := *h

	// make a deep copy of the scope definitions
	h2.scopes = make([]scope, len(h.scopes))
	copy(h2.scopes, h.scopes)

	// make a deep copy of the of the encoder whose buffer is holding the
	// pre-encoded attrs k-v pairs
	h2.preEnc = h.preEnc.DeepCopy()

	return &h2
}

func (h *Handler) debug(format string, args ...any) {
	if !h.Verbose {
		return
	}
	InternalLogger().Printf(format, args...)
}

// Enabled reports whether the handler handles records at the given level. The
// handler ignores records whose level is lower. It is called early, before any
// arguments are processed, to save effort if the log event should be discarded.
// If called from a Logger method, the first argument is the context passed to
// that method, or context.Background() if nil was passed or the method does not
// take a context. The context is passed so Enabled can use its values to make a
// decision.
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.Level.Level()
}

// Handle handles the Record. It will only be called when Enabled returns true.
// The Context argument is as for Enabled. It is present solely to provide
// Handlers access to the context's values. Canceling the context does not
// affect record processing.
//
// Handle methods that produce output should observe the following rules:
//   - If r.Time is the zero time, ignore the time.
//   - If r.PC is zero, ignore it.
//   - Attr's values should be resolved.
//   - If an Attr's key and value are both the zero value, ignore the Attr.
//   - If a group's key is empty, inline the group's Attrs.
//   - If a group has no Attrs (even if it has a non-empty key),
//     ignore it.
//
// Additional rules are taken from test cases in `slogtest.go`. Handler passes
// all conformance tests in `testing/slogtest.go` except one. For "If r.Time is
// the zero time, ignore the time.", the test suite interprets "ignore" as "omit
// the time entirely." This is not acceptable in most logging protocols,
// including Fluent. Rather than interpretint "ignore" as "omit", Handler
// *ignores* zero values and uses time.Now() as a reasonable fallback. The
// modified test suite is included in `slogtest.go`.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {

	// helper for serialization error collection
	errs := new(encErrs)

	// rule: ignore record time if zero
	t := r.Time
	if t.IsZero() {
		t = time.Now()
	}

	// get buffer/encoder into which we directly serialize attrs
	enc := h.logPool.Get()

	// encode the log timestamp
	err := enc.EncodeEventTime(t)
	if err != nil {
		return fmt.Errorf("failed to Handle slog request: %w", err)
	}

	// copy so we don't affect future calls
	scopes := append(h.scopes[:0:0], h.scopes...)

	// log level and message fields added the top scope
	scopes[0].nAttrs += 2

	// rule: ignore source if no program counter, else add to top scope
	addSrc := h.AddSource && r.PC != 0
	if addSrc {
		scopes[0].nAttrs++
	}

	// slog record attrs added to the last scope
	scopes[len(scopes)-1].nAttrs += r.NumAttrs()

	// rule: remove empty groups and adjust parent scope lengths
	for i := len(scopes) - 1; i > 0; i-- {
		if scopes[i].nAttrs == 0 {
			scopes[i-1].nAttrs--
			scopes = scopes[:i]
		} else {
			// ancestors contain this non-empty scope -> can't be empty
			break
		}
	}
	nScopes := len(scopes)
	lastScopeIdx := nScopes - 1

	// track where we start map length headers in case we have to backtrack and
	// adjust them downward (e.g. skipping attrs or skipped attrs in a child
	// group resulting in the group being empty and thus removed)
	sHdrIdx := make([]int, nScopes)

	// track the original lengths, so we know if we have to adjust
	sLen := make([]int, nScopes)

	// process each scope
	for i, s := range scopes {
		if i > 0 {
			// encode the group name, which lands at the tail of the parent,
			// whose own length header accounts for the key:map pair; the top
			// scope does not have a name - it's in the outer msgpack array
			errs.join(fmt.Sprintf("group key: %s", s.key), enc.EncodeString(s.key))
		}

		// store where the map length header begins, and the originally encoded length
		sHdrIdx[i] = enc.Len()
		sLen[i] = s.nAttrs

		// overallocate (by up to 1 byte) for the map length header for the top
		// and last scopes; they have dynamic attrs that can expand if subattrs
		// are groups with empty keys, resulting in inlining of grandchild
		// attrs; this allows us to adjust to expansion without extra copying
		if i == 0 || i == lastScopeIdx {
			errs.join("record length", enc.EncodeMapLen(s.nAttrs))
		} else {
			errs.join("record length", h.encodeMap16Len(enc, s.nAttrs))
		}

		// augment the top scope with fixed attrs
		if i == 0 {
			// serialize the log level into the top scope
			errs.join("log level key", enc.EncodeString(slog.LevelKey))
			errs.join("log level value", enc.EncodeString(r.Level.String()))

			// serialize the source information to the top scope
			if addSrc {
				fs := runtime.CallersFrames([]uintptr{r.PC})
				f, _ := fs.Next()
				errs.join("source key", enc.EncodeString(slog.SourceKey))
				errs.join("source info", enc.EncodeString(fmt.Sprintf("%s:%d", f.File, f.Line)))
			}

			// serialize the message into the top scope
			errs.join("message key", enc.EncodeString(slog.MessageKey))
			errs.join("log message", enc.EncodeString(r.Message))

			// slog.Attrs passed in via the ctx also go to the top scope
			if ctxAttr, ok := ctx.Value(ContextKey).(slog.Attr); ok {
				n, err := h.encodeAttr(enc, ctxAttr)
				sLen[i] += n
				errs.join("context attrs", err)
			}
		}

		if i != lastScopeIdx {

			// add the pre-serialized attr bytes
			enc.Write(h.preEnc.Bytes()[s.offset:scopes[i+1].offset])

		} else {

			// add the pre-serialized attr bytes
			_, err := enc.Write(h.preEnc.Bytes()[s.offset:])
			errs.err = errors.Join(errs.err, err)

			// add attrs from the slog.Record to the final/deepest scope
			r.Attrs(func(attr slog.Attr) bool {
				n, err := h.encodeAttr(enc, attr)

				// account for skipped or expanded attrs
				if n != 1 {
					sLen[i] += n - 1
				}
				errs.join("record attr", err)
				return true // continue iterating
			})
		}

	}

	// make adjustments to the map length headers for adjusted groups
	for i := 0; i < nScopes; i++ {
		if sLen[i] != scopes[i].nAttrs {
			adjustMapLenHeader(enc.Bytes(), sHdrIdx[i], scopes[i].nAttrs, sLen[i])
		}
	}

	// record all serialization errors
	if errs.err != nil {
		InternalLogger().Printf("encoding errors in Handle:\n%v", errs.err)
	}

	h.client.Send(enc)

	return nil
}

// encErrs collects serialization errors
type encErrs struct {
	err error
}

func (e *encErrs) join(target string, err error) (wasErr bool) {
	if err == nil {
		return false
	}
	e.err = errors.Join(e.err, fmt.Errorf("failed to encode %s: %w", target, err))
	return true
}

func (h *Handler) encodeAttr(enc *Encoder, attr slog.Attr) (nEncoded int, err error) {

	// rule: must first resolve, and then ignore if empty
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return 0, nil
	}

	k, v := attr.Key, attr.Value

	// collect serialization errors
	errs := new(encErrs)

	// encode the key, ignoring groups with their divergent handling
	if v.Kind() != slog.KindGroup {
		if len(k) == 0 {
			// rule: ignore non-group attrs with empty keys
			return 0, nil
		}
		errs.join("attr key", enc.EncodeString(k))
		nEncoded = 1

		// encode the value
		switch vk := v.Kind(); vk {
		case slog.KindAny:
			errs.join("slog.KindAny for key: "+k, enc.Encode(v.Any()))
		case slog.KindBool:
			errs.join("slog.KindBool for key: "+k, enc.EncodeBool(v.Bool()))
		case slog.KindDuration:
			errs.join("slog.KindDuration for key: "+k, enc.EncodeDuration(v.Duration()))
		case slog.KindFloat64:
			errs.join("slog.KindFloat64 for key: "+k, enc.EncodeFloat64(v.Float64()))
		case slog.KindInt64:
			errs.join("slog.KindInt64 for key: "+k, enc.EncodeInt64(v.Int64()))
		case slog.KindString:
			errs.join("slog.KindString for key: "+k, enc.EncodeString(v.String()))
		case slog.KindTime:
			errs.join("slog.KindTime for key: "+k, enc.EncodeString(v.Time().Format(h.TimeFormat)))
		case slog.KindUint64:
			errs.join("slog.KindUint64 for key: "+k, enc.EncodeUint64(v.Uint64()))
		case slog.KindLogValuer:
			errs.join("slog.KindLogValuer for key: "+k, errors.New("Value.Resolve() invariant violation"))
		case slog.KindGroup:
			break // unreachable
		default:
			errs.join("Value for key: "+k, fmt.Errorf("unknown slog.Value.Kind: %d", vk))
		}

		return 1, errs.err
	}

	// differs from WithGroup, with variable size; this is for static groups:
	// slog.Group attr;
	//   example:
	//   logger.LogAttrs(level, msg, slog.Group("s",
	//       slog.Int("a", 1),
	//       slog.Int("b", 2),
	//   ))
	gAttrs := v.Group()
	gLen := len(gAttrs)

	// rule: ignore empty groups entirely (eject before encoding key)
	if gLen == 0 {
		return 0, nil
	}

	// rule: inline attrs if key is empty
	if len(k) == 0 {
		for i := 0; i < len(gAttrs); i++ {
			n, e := h.encodeAttr(enc, gAttrs[i])
			nEncoded += n
			errs.join("attr value for group with key: "+k, e)
		}
		return nEncoded, errs.err
	}

	// track where the group starts, in case we end up skipping every attr,
	// making it empty and therefore requiring we omit it entirely
	gIdx := enc.Buffer.Len()

	// encode group key
	errs.join("attr key", enc.EncodeString(k))

	// track where we write the length header, in case we have to adjust it
	// after skipping attrs in the group
	gLenIdx := enc.Buffer.Len()

	// encode map len; allocate 2 bytes; handles cases where length expands when
	// child attrs are groups with zero-length keys, resulting in grandchildren
	// getting inlined; we waste 1 byte when the final length is less than 16,
	// but this allows us to update the length without any copying
	errs.join("map len header for group: "+k, h.encodeMap16Len(enc, gLen))

	nAdded := 0
	for i := 0; i < gLen; i++ {
		// { k0: <- current scope
		//       {k1:
		//           {kv3, kv4, kv5},
		//       {kv2}
		// }
		// if k1 is empty, then attrs kv3, kv4, and kv5 become part of g0
		n, e := h.encodeAttr(enc, gAttrs[i])
		nAdded += n
		errs.join("attr value for group with key: "+k, e)
	}

	// omit this if all attrs omitted and we have an empty group now
	if nAdded == 0 {
		enc.Buffer.Truncate(gIdx)
	}

	// we have some attrs, but not the number expected
	if nAdded != gLen {
		adjustMapLenHeader(enc.Buffer.Bytes(), gLenIdx, gLen, nAdded)
	}

	return 1, errs.err
}

func adjustMapLenHeader(b []byte, idx, oldLen, newLen int) {
	if oldLen < 16 {
		b[idx] = msgpcode.FixedMapLow | byte(newLen)
	} else if oldLen < math.MaxUint16 {
		n := uint16(newLen)
		// leave the size header byte at idx+0 as msgpcode.Map16
		b[idx+1] = byte(n >> 8)
		b[idx+2] = byte(n)
	} else {
		n := uint32(newLen)
		// leave the size header byte at idx+0 as msgpcode.Map32
		b[idx+1] = byte(n >> 24)
		b[idx+2] = byte(n >> 16)
		b[idx+3] = byte(n >> 8)
		b[idx+4] = byte(n)
	}
}

// WithAttrs returns a new Handler whose attributes consist of both the
// receiver's attributes and the arguments. The Handler owns the slice: it may
// retain, modify or discard it.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {

	// rule: skip if no attrs
	if len(attrs) == 0 {
		return h
	}

	// make independent copy
	h2 := h.deepCopy()

	// update metadata for the current scope *in the new logger*
	idx := len(h2.scopes) - 1

	// gather serialization errors
	errs := new(encErrs)

	// count number of added attrs
	added := 0

	// encode all the attrs
	for i := 0; i < len(attrs); i++ {
		n, err := h2.encodeAttr(h2.preEnc, attrs[i])
		added += n
		errs.join("attr in WithAttrs", err)
	}

	if errs.err != nil {
		InternalLogger().Println(errs)
	}

	// if none added, don't create a new scope
	if added == 0 {
		return h
	}

	// account for new attrs
	h2.scopes[idx].nAttrs += added

	return h2
}

// WithGroup returns a new Handler with the given group appended to the
// receiver's existing groups, increasing the nesting level within the msgpack
// map serialization of the Fluent log record.
//
// The new scope ends at the end of the log event. That is,
//
//	logger.WithGroup("s").LogAttrs(level, msg, slog.Int("a", 1), slog.Int("b", 2))
//
//	behaves like
//
//	logger.LogAttrs(level, msg, slog.Group("s", slog.Int("a", 1), slog.Int("b", 2)))
//
// If the name is empty, WithGroup returns the receiver, which results in the
// nested attributes being inlined into the parent scope.
func (h *Handler) WithGroup(name string) slog.Handler {

	// rule: ignore if name is empty (true for any attr)
	if len(name) == 0 {
		return h
	}

	// make an independent copy of the logger
	h2 := h.deepCopy()

	// update the count of the parent, for attr name:map for this group
	h2.scopes[len(h2.scopes)-1].nAttrs++

	// add the new scope
	h2.scopes = append(
		h2.scopes,
		scope{key: name, offset: h2.preEnc.Len()},
	)

	return h2
}

// encodeMap16Len encodes map length as 16-bit integers even if the length would
// fit in an 8-byte integer. This is useful for scopes with dynamically encoded
// attrs, which could include groups with attrs that are also groups but have
// empty keys, causing their attrs to get inlined into the parent, increasing
// its length.
func (h *Handler) encodeMap16Len(enc *Encoder, mlen int) error {
	if mlen > math.MaxUint16 {
		return errors.New("exceeded maximum map length for one log group/scope (65535)")
	}
	n := uint16(mlen)
	h.buf3[0] = msgpcode.Map16
	h.buf3[1] = byte(n >> 8)
	h.buf3[2] = byte(n)
	_, err := enc.Write(h.buf3[:])
	return err
}
