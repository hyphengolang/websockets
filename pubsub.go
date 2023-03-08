package websockets

import (
	"github.com/hyphengolang/websockets/pkg/structures"
)

type PSubcriber interface {
	Publisher
	Subscriber
	// Set registers a new connection
	// it returns a function that unregisters the connection
	Set(conn *connHandler) (unset func())
}

type Publisher interface {
	// Publish sends a message to all connections
	Publish(msg *Message) error
}

type Subscriber interface {
	// Subscribe returns a channel that receives messages from all connections
	Subscribe() <-chan *Message
}

var _ PSubcriber = (*psub)(nil)

type psub struct {
	// broadcast is the channel that receives messages from the server
	broadcast chan *Message
	// register is the channel that registers new connections
	register chan *connHandler
	// unregister is the channel that unregisters connections
	unregister chan *connHandler
	// connections is the list of connections
	connections *structures.SyncMap[string, structures.Set[*connHandler]]
}

func (s *psub) Set(conn *connHandler) (unset func()) {
	s.register <- conn
	return func() { s.unregister <- conn }
}

// Publish implements PSubcriber
func (s *psub) Publish(msg *Message) error {
	s.broadcast <- msg
	return nil
}

// Subscribe implements PSubcriber
func (s *psub) Subscribe() <-chan *Message {
	return s.broadcast
}

func (s *psub) listen() {
	for {
		select {
		case conn := <-s.register:
			if conns, ok := s.connections.Load(conn.channel); ok {
				conns.Add(conn)
			} else {
				s.connections.Store(conn.channel, structures.NewSet(conn))
			}
		case conn := <-s.unregister:
			if conns, ok := s.connections.Load(conn.channel); ok {
				conns.Remove(conn)
			}
		case msg := <-s.Subscribe():
			if conns, ok := s.connections.Load(msg.Channel); ok {
				for conn := range conns {
					select {
					case conn.rcv <- msg:
					default:
						conns.Remove(conn)
					}
				}
			}
		}
	}
}

func NewSubscriber() PSubcriber {
	ps := psub{
		broadcast:   make(chan *Message, 256),
		register:    make(chan *connHandler),
		unregister:  make(chan *connHandler),
		connections: structures.NewSyncMap[string, structures.Set[*connHandler]](),
	}
	go ps.listen()
	return &ps
}
