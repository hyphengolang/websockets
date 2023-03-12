// A websocket client that connects to a NATS server and subscribes to a topic.
package websockets

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/nats-io/nats.go"
)

type Option func(*Client)

func WithWaitTimes(writeWait, pongWait time.Duration) Option {
	return func(c *Client) {
		c.writeWait = writeWait
		c.pongWait = pongWait
		c.pingPeriod = (pongWait * 9) / 10
	}
}

var _ http.Handler = (*Client)(nil)

type Client struct {
	// nats is the NATS connection. It is used to publish and subscribe to NATS topics.
	nats *nats.EncodedConn
	// writeWait is time allowed to write a message to the peer.
	writeWait time.Duration
	// pongWait is the time allowed to read the next pong message from the peer.
	pongWait time.Duration
	// pingPeriod is the period for which pings to peer with this pingPeriod. Must be less than pongWait.
	pingPeriod time.Duration
}

func NewClient(nc *nats.Conn, opts ...Option) (*Client, error) {
	ec, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {
		return nil, fmt.Errorf("failed to create encoded connection: %w", err)
	}

	c := &Client{
		nats: ec,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.writeWait == 0 {
		c.writeWait = 10 * time.Second
	}

	if c.pongWait == 0 {
		c.pongWait = 60 * time.Second
	}

	if c.pingPeriod == 0 {
		c.pingPeriod = (c.pongWait * 9) / 10
	}

	return c, err
}

// ServeHTTP implements http.Handler
func (c *Client) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	subject, _ := subjectFromContext(r.Context())
	if subject == "" {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("failed to get subject from context"))
		return
	}

	send, recv, err := makeChans(subject, c.nats)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("failed to bind chans: %v", err)))
		return
	}

	wc, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("ws upgrade failed"))
		return
	}
	defer wc.Close()

	conn := &connHandler{wc, ws.OpText}

	go c.read(conn, send)
	c.write(conn, recv)
}

func (c *Client) read(wc *connHandler, send chan<- *Message) {
	defer wc.Close()

	_ = wc.readDeadline(c.pongWait)

	for {
		msg, err := wc.readMessage(c.pongWait)
		if err != nil {
			log.Printf("failed to read message: %v", err)
			return
		}

		send <- &Message{Payload: msg}
	}
}

func (c *Client) write(wc *connHandler, recv <-chan *Message) {
	ticker := time.NewTicker(c.pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-recv:
			if !ok {
				_ = wc.writeMessage(c.writeWait, ws.OpClose, nil)
				return
			}

			if err := wc.writeText(c.writeWait, msg.Payload); err != nil {
				log.Printf("failed to write message: %v", err)
				return
			}
		case <-ticker.C:
			if err := wc.writeMessage(c.writeWait, ws.OpPing, []byte("ping")); err != nil {
				log.Printf("failed to write ping: %v", err)
				return
			}
		}
	}
}

type connHandler struct {
	rwc  net.Conn
	want ws.OpCode
}

func (c *connHandler) Close() error {
	log.Printf("channel closed")
	return c.rwc.Close()
}

func (c *connHandler) readDeadline(t time.Duration) error {
	return c.rwc.SetReadDeadline(time.Now().Add(t))
}

func (c *connHandler) writeDeadline(t time.Duration) error {
	return c.rwc.SetWriteDeadline(time.Now().Add(t))
}

func (c *connHandler) readMessage(t time.Duration) ([]byte, error) {
	controlHandler := wsutil.ControlFrameHandler(c.rwc, ws.StateServerSide)
	rd := wsutil.Reader{
		Source:          c.rwc,
		State:           ws.StateServerSide,
		CheckUTF8:       true,
		SkipHeaderCheck: false,
		OnIntermediate:  controlHandler,
	}

	for {
		hdr, err := rd.NextFrame()
		if err != nil {
			return nil, err
		}

		if hdr.OpCode.IsControl() {
			if /* handle pong messages */ hdr.OpCode == ws.OpPong {
				_ = c.readDeadline(t)
				log.Println("set read deadline")
			}

			if err := controlHandler(hdr, &rd); err != nil {
				return nil, err
			}
			continue
		}

		if hdr.OpCode&(c.want) == 0 {
			if err := rd.Discard(); err != nil {
				return nil, err
			}
			continue
		}

		bts, err := io.ReadAll(&rd)

		return bts, err
	}
}

func (c *connHandler) writeText(t time.Duration, p []byte) error {
	return c.writeMessage(t, ws.OpText, p)
}

func (c *connHandler) writeMessage(t time.Duration, op ws.OpCode, p []byte) error {
	if err := c.writeDeadline(t); err != nil {
		return err
	}

	return wsutil.WriteServerMessage(c.rwc, op, p)
}

// Must be able to be gob encoded
type Message struct {
	Method string
	// TODO this should be a json.RawMessage
	Payload []byte
}

// makeChans binds a send and receive channel to a NATS subject.
func makeChans(subject string, nc *nats.EncodedConn) (chan<- *Message, <-chan *Message, error) {
	send := make(chan *Message, 64)
	if err := nc.BindSendChan(subject, send); err != nil {
		return nil, nil, fmt.Errorf("failed to bind send channel: %w", err)
	}

	recv := make(chan *Message, 64)
	if _, err := nc.BindRecvChan(subject, recv); err != nil {
		return nil, nil, fmt.Errorf("failed to bind receive channel: %w", err)
	}

	return send, recv, nil
}

type contextKey struct{ string }

func (c contextKey) String() string {
	return "websockets context key " + c.string
}

var (
	subjectKey = contextKey{"subject-key"}
)

func WithSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, subjectKey, subject)
}

func subjectFromContext(ctx context.Context) (string, bool) {
	subject, ok := ctx.Value(subjectKey).(string)
	return subject, ok
}
