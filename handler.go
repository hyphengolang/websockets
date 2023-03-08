package websockets

type Handler interface {
	Serve(w ResponseWriter, p *Payload)
}

type HandlerFunc func(w ResponseWriter, p *Payload)

func (hf HandlerFunc) Serve(w ResponseWriter, p *Payload) {
	hf(w, p)
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
