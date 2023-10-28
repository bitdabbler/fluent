package fluent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/vmihailenco/msgpack/v5"
	"github.com/vmihailenco/msgpack/v5/msgpcode"
)

type TestMessage struct {
	Tag    string
	Time   time.Time
	Record map[string]any
	Option map[string]any
}

// DecodeMsgpack deserializes the payload, which is expected to conform to the
// Fluent Message event mode format.
// [
//
//	 	tag<string>,
//		time<EventTime | int>,
//		record<map[string]any>,
//		option<optional map[string]any>
//
// ]
func (m *TestMessage) DecodeMsgpack(dec *msgpack.Decoder) error {

	n, err := dec.DecodeArrayLen()
	if err != nil {
		return fmt.Errorf("failed to decode outer message array length: %v", err)
	}

	// decode the tag
	err = dec.Decode(&m.Tag)
	if err != nil {
		return fmt.Errorf("failed to decode tag field: %v", err)
	}

	// decode the timestamp
	typeCode, err := dec.PeekCode()
	if err != nil {
		return fmt.Errorf("failed to read type code for the time field: %v", err)
	}
	switch typeCode {
	case msgpcode.FixExt8:
		et := EventTime{}
		err = dec.Decode(&et)
		if err != nil {
			return fmt.Errorf("failed to decode the time field: %v", err)
		}
		m.Time = time.Time(et)
	case msgpcode.Int64:
		unix, err := dec.DecodeInt64()
		if err != nil {
			return fmt.Errorf("failed to decode the time field: %v", err)
		}
		m.Time = time.Unix(unix, 0)
	}

	// decode the record
	err = dec.Decode(&m.Record)
	if err != nil {
		return fmt.Errorf("failed to decode the record field: %v", err)
	}

	if n == 4 {
		// decode the option field
		err = dec.Decode(&m.Option)
		if err != nil {
			return fmt.Errorf("failed to decode the option field: %v", err)
		}
	}
	return nil
}

type testServer struct {
	listener   net.Listener
	messageCh  chan *TestMessage
	network    string
	host       string
	port       int
	shutdownCh chan struct{}
	*testServerOptions
}

const testHost = "127.0.0.1"
const testTag = "test-tag"

type testServerOptions struct {
	verbose bool
}

func newTestServer(opts *testServerOptions) (*testServer, error) {
	if opts == nil {
		opts = &testServerOptions{}
	}

	s := &testServer{
		messageCh:         make(chan *TestMessage, 128),
		shutdownCh:        make(chan struct{}),
		network:           "tcp",
		host:              testHost,
		testServerOptions: opts,
	}

	// assign port dynamically (use port 0 to assign dynamically)
	l, err := net.Listen(s.network, s.host+":0")
	if err != nil {
		return nil, fmt.Errorf("failed to start test server listener: %v", err)
	}
	s.listener = l

	// parse out the dynamically assigned port
	addr := l.Addr().String()
	idx := strings.LastIndex(addr, ":")
	if idx == len(addr)-1 {
		return nil, errors.New("bad addr: ends with ':'")
	}
	s.port, err = strconv.Atoi(addr[idx+1:])
	if err != nil {
		return nil, fmt.Errorf("invalid port value: '%s': %v", addr[idx+1:], err)
	}

	// start the server loop
	go func() {
		s.debug("starting listener")
		for {
			select {
			case <-s.shutdownCh:
				s.debug("shutting down")
				s.listener.Close()
				return
			default:
				conn, err := l.Accept()
				if err != nil {
					s.debug("listener.Accept() error: %v", err)
					continue
				}
				s.debug("new client connected")
				go s.handle(conn)
			}
		}
	}()

	return s, nil
}

func (s *testServer) Shutdown() {
	close(s.shutdownCh)
}

func (s *testServer) handle(conn net.Conn) {
	d := msgpack.NewDecoder(conn)

	for {
		// only supporting msg mode for now
		m := new(TestMessage)
		err := d.Decode(m)
		if err != nil {
			s.debug("failed to decode Fluent Message: %v\n", err)
			break
		}
		s.messageCh <- m
	}

	s.debug("closing connection")
	conn.Close()
}

func (ts *testServer) debug(format string, args ...any) {
	if !ts.verbose {
		return
	}
	InternalLogger().Printf("testServer: "+format, args...)
}

// testClient is a test client that records all messages rather than send them
// to a server. It implements the Handler's Sink interface.
type testClient struct {
	logs []*TestMessage
}

func newTestClient() *testClient {
	return &testClient{logs: make([]*TestMessage, 0)}
}

func (c *testClient) Send(enc *Encoder) {
	dec := msgpack.NewDecoder(bytes.NewBuffer(enc.Bytes()))
	m := new(TestMessage)
	if err := dec.Decode(m); err != nil {
		log.Fatalf("test client Send() failed to decode message: %v", err)
	}
	c.logs = append(c.logs, m)
}

func (c *testClient) Shutdown(ctx context.Context) error {
	return nil
}

func (c *testClient) recordsWithTimeInlined() []map[string]any {
	res := make([]map[string]any, len(c.logs))
	for i := 0; i < len(c.logs); i++ {
		res[i] = c.logs[i].Record
		res[i][slog.TimeKey] = c.logs[i].Time
	}
	return res
}
