package sync

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"sync-engine/internal/protocol"
)

const (
	// writeWait is the deadline for a single WriteMessage/WriteControl call.
	writeWait = 10 * time.Second

	// pongWait is how long we'll wait for a pong before considering the connection dead.
	pongWait = 60 * time.Second

	pingInterval = (pongWait * 9) / 10

	// sendBufferSize bounds the outbound queue per connection.
	sendBufferSize = 256
)

// Client wraps one WebSocket connection.
type Client struct {
	id   string
	conn *websocket.Conn

	send chan []byte

	closeOnce sync.Once
	closed    chan struct{}
}

// NewClient wraps conn as a Client identified by id
func NewClient(id string, conn *websocket.Conn) *Client {
	return &Client{
		id:     id,
		conn:   conn,
		send:   make(chan []byte, sendBufferSize),
		closed: make(chan struct{}),
	}
}

// ClientID implements RoomClient.
func (c *Client) ClientID() string { return c.id }

// Enqueue implements RoomClient.
func (c *Client) Enqueue(env protocol.Envelope) bool {
	data, err := protocol.Encode(env)
	if err != nil {
		return false
	}

	select {
	case <-c.closed:
		return false
	default:
	}

	select {
	case c.send <- data:
		return true
	default:
		return false
	}
}

// Close closes the connection, sending a WebSocket close frame with reason first when possible.
func (c *Client) Close(reason string) {
	c.closeOnce.Do(func() {
		close(c.closed)
		_ = c.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, reason),
			time.Now().Add(writeWait),
		)
		c.conn.Close()
	})
}
