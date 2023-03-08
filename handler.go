package websockets

import "encoding/json"

type Handler interface {
	Serve(w ResponseWriter, p *Payload)
}

type HandlerFunc func(w ResponseWriter, p *Payload)

func (hf HandlerFunc) Serve(w ResponseWriter, p *Payload) {
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
		Payload: Payload{
			Method: method,
			Data:   json.RawMessage(p),
		},
	}

	r.rwc.rcv <- msg
}

func (r *response) Broadcast(method string, p []byte) error {
	var msg = &Message{
		Channel: r.rwc.channel,
		Payload: Payload{
			Method: method,
			Data:   json.RawMessage(p),
		},
	}

	return r.ps.Publish(msg)
}

func (c *Client) On(method string, hf HandlerFunc) {
	if c.m == nil {
		panic("websocket: client mux is nil")
	}

	e := muxEntry{method, HandlerFunc(hf)}

	if _, ok := c.m[method]; ok {
		panic("websocket: client mux entry already exists")
	}

	c.m[method] = e
}

func (c *Client) match(method string) (Handler, bool) {
	if c.m == nil {
		panic("websocket: client mux is nil")
	}

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
