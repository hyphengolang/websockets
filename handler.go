package websockets

import (
	"encoding/json"
	"strings"
)

type Handler interface {
	Serve(w ResponseWriter, p *Message)
}

type HandlerFunc func(w ResponseWriter, p *Message)

func (hf HandlerFunc) Serve(w ResponseWriter, p *Message) {
	hf(w, p)
}

type ResponseWriter interface {
	Broadcast(method string, p []byte) error
	Send(method string, p []byte)
}

var _ ResponseWriter = (*response)(nil)

type response struct {
	// channel string
	rwc *connHandler
	ps  Publisher
}

func (r *response) Send(method string, p []byte) {
	msg := &Message{
		Channel: r.rwc.channel,
		Data: Data{
			Method:  method,
			Payload: json.RawMessage(p),
		},
	}

	r.rwc.rcv <- msg
}

func (r *response) Broadcast(method string, p []byte) error {
	var msg = &Message{
		Channel: r.rwc.channel,
		Data: Data{
			Method:  method,
			Payload: json.RawMessage(p),
		},
	}

	return r.ps.Publish(msg)
}

func (c *Client) On(method string, hf HandlerFunc) {
	if c.m == nil {
		panic("websocket: client mux is nil")
	}

	// NOTE -- vunerable to timing attacks
	method = strings.ToLower(method)

	e := muxEntry{method, hf}

	if _, ok := c.m[method]; ok {
		panic("websocket: client mux entry already exists")
	}

	c.m[method] = e
}

func (c *Client) match(method string) (Handler, bool) {
	if c.m == nil {
		panic("websocket: client mux is nil")
	}

	// NOTE -- vunerable to timing attacks
	method = strings.ToLower(method)

	e, ok := c.m[method]
	if !ok {
		return nil, false
	}

	return e.h, true
}

type muxEntry struct {
	method string
	h      Handler
}
