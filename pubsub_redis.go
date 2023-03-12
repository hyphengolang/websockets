package websockets

import (
	"context"
	"encoding/json"
	"log"

	"github.com/hyphengolang/websockets/pkg/structures"
	"github.com/redis/go-redis/v9"
)

var _ PSubcriber = (*predis)(nil)

type predis struct {
	// r is the redis client
	r *redis.Client
	// sub is the redis pubsub client
	sub *redis.PubSub
	// broadcast is the channel that receives messages from the server
	broadcast chan *Message
	// register is the channel that registers new connections
	register chan *connHandler
	// unregister is the channel that unregisters connections
	unregister chan *connHandler
	// connections is the redis pubsub client
	connections *structures.SyncMap[string, structures.Set[*connHandler]]
}

// Publish implements PSubcriber
func (s *predis) Publish(msg *Message) error {
	ctx := context.Background()
	return s.r.Publish(ctx, msg.Channel, msg.Data).Err()
}

// Subscribe implements PSubcriber
// currently not implemented
func (s *predis) Subscribe() <-chan *Message {
	panic("not implemented")
}

// Set implements PSubcriber
func (s *predis) Set(conn *connHandler) (unset func()) {
	s.register <- conn
	return func() { s.unregister <- conn }
}

// newRSubscriber returns a new PSubcriber
// using redis as the backend
func newRSubscriber(r *redis.Client) PSubcriber {
	p := &predis{
		r:           r,
		sub:         r.Subscribe(context.Background()),
		broadcast:   make(chan *Message, 256),
		register:    make(chan *connHandler),
		unregister:  make(chan *connHandler),
		connections: structures.NewSyncMap[string, structures.Set[*connHandler]](),
	}
	go p.listen()
	return p
}

func (s *predis) listen() {
	for {
		select {
		case conn := <-s.register:
			if conns, ok := s.connections.Load(conn.channel); ok {
				conns.Add(conn)
			} else {
				s.connections.Store(conn.channel, structures.NewSet(conn))
				s.sub.Subscribe(context.Background(), conn.channel)
			}
		case conn := <-s.unregister:
			if conns, ok := s.connections.Load(conn.channel); ok {
				conns.Remove(conn)
				// if there are no more connections
				// unsubscribe from the channel
			}
		case msg := <-s.sub.Channel():
			var p Data
			if err := json.Unmarshal([]byte(msg.Payload), &p); err != nil {
				// if error, then connection is lost
				log.Printf("error unmarshaling payload: %v", err)
				continue
			}

			if conns, ok := s.connections.Load(msg.Channel); ok {
				for conn := range conns {
					select {
					case conn.rcv <- &Message{
						Channel: msg.Channel,
						Data:    p,
					}:
					default:
						conns.Remove(conn)
					}
				}
			}
		}
	}
}
