package sync

import (
	"context"
	"time"

	"sync-engine/internal/protocol"
)

// Dispatcher processes one decoded, validated envelope received from a
// client, routing it to whatever room/document logic it implies.
type Dispatcher func(ctx context.Context, from *Client, env protocol.Envelope)

// ReadPump reads and processes messages from the client's connection
// until it closes or a read error occurs.
func (c *Client) ReadPump(ctx context.Context, dispatch Dispatcher) {
	c.conn.SetReadLimit(protocol.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		env, err := protocol.Decode(raw)
		if err != nil {
			errEnv, buildErr := buildErrorEnvelope(err)
			if buildErr == nil {
				c.Enqueue(errEnv)
			}
			// A malformed message doesn't kill the connection by itself --
			// only a transport-level read error (handled above) does.
			continue
		}

		dispatch(ctx, c, env)
	}
}
