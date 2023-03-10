package websockets

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/redis/go-redis/v9"
)

type Message struct {
	Channel string `json:"channel"`
	Data    Data   `json:"data"`
}

// Data is the data sent to the client
type Data struct {
	Method  string          `json:"method"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func (p Data) MarshalBinary() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Data) UnmarshalBinary(b []byte) error {
	return json.Unmarshal(b, p)
}

func read(conn *connHandler, c *Client) {
	defer conn.Close()

	// if handler exists for connect, call it
	{
		msg := &Message{
			Channel: conn.channel,
			Data:    Data{Method: "connect"},
		}

		if h, match := c.match(msg.Data.Method); match {
			h.Serve(&response{conn, c.ps}, msg)
		}
	}

	// TODO -- if handler exists for disconnect, call it
	{

	}

	for {
		s, err := wsutil.ReadClientText(conn.rwc)
		if err != nil {
			log.Printf("read err: %v", err)
			return
		}

		// FIXME if message isn't JSON, log and continue
		var p Data
		if err := json.Unmarshal(s, &p); err != nil {
			log.Printf("json err: %v", err)
			return
		}

		h, match := c.match(p.Method)
		if !match {
			log.Printf("no handler for %s", p.Method)
			return
		}

		msg := &Message{
			Channel: conn.channel,
			Data:    p,
		}

		h.Serve(&response{conn, c.ps}, msg)
	}
}

// close, write
func write(conn *connHandler) {
	defer conn.Close()

	for msg := range conn.rcv {
		p, err := json.Marshal(msg.Data)
		if err != nil {
			log.Printf("json err: %v", err)
			return
		}

		if err := wsutil.WriteServerText(conn.rwc, p); err != nil {
			log.Printf("write err: %v", err)
			return
		}
	}
}

var _ http.Handler = (*Client)(nil)

// Client is a websocket client
// currently only supports text messages
type Client struct {
	// u upgrades the HTTP request to a websocket connection
	u ws.HTTPUpgrader
	// ps handles publishing messages to all connections
	ps PSubcriber
	// m is the map of muxEntries
	m map[string]muxEntry
}

// ServeHTTP implements http.Handler
func (c *Client) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	channel, ok := FromContext(r.Context())
	if !ok {
		http.Error(w, "channel not found", http.StatusBadRequest)
		return
	}

	if channel == "" {
		http.Error(w, "channel is empty", http.StatusBadRequest)
		return
	}

	rwc, _, _, err := c.u.Upgrade(r, w)
	if err != nil {
		return
	}

	conn := connHandler{
		rwc:     rwc,
		channel: channel,
		rcv:     make(chan *Message, 256),
	}

	unset := c.ps.Set(&conn)
	defer unset()

	go read(&conn, c)
	write(&conn) // I don't think this needs to be in a goroutine
}

func NewClient(opts ...Option) *Client {
	c := Client{
		m: make(map[string]muxEntry),
	}

	for _, opt := range opts {
		opt(&c)
	}

	if c.ps == nil {
		c.ps = NewSubscriber()
	}

	return &c
}

type Option func(*Client)

func WithRedis(r *redis.Client) Option {
	return func(c *Client) {
		c.ps = newRSubscriber(r)
	}
}

var _ io.ReadWriteCloser = (*connHandler)(nil)

type connHandler struct {
	// rwc is the underlying websocket connection
	rwc net.Conn
	// channel that the connection is subscribed to
	channel string
	// rcv is the channel that receives messages from the connection
	rcv chan *Message
}

// Read implements io.ReadWriteCloser
func (c *connHandler) Read(p []byte) (n int, err error) {
	return c.rwc.Read(p)
}

// Write implements io.ReadWriteCloser
func (c *connHandler) Write(p []byte) (n int, err error) {
	return c.rwc.Write(p)
}

func (c *connHandler) Close() error {
	if err := c.rwc.Close(); err != nil {
		return err
	}
	close(c.rcv)
	return nil
}

type Context string

var (
	channelKey Context = "channel"
)

func NewContext(ctx context.Context, channel string) context.Context {
	return context.WithValue(ctx, channelKey, channel)
}

func FromContext(ctx context.Context) (string, bool) {
	channel, ok := ctx.Value(channelKey).(string)
	return channel, ok
}
