package sync

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"sync-engine/internal/protocol"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// joinDeadline bounds how long a newly-upgraded connection has to send
// its join_document message before the server gives up on it.
const joinDeadline = 10 * time.Second

// Server ties together everything this package builds
type Server struct {
	Connections *ConnectionManager
	Rooms       *RoomManager
}

// NewServer constructs a Server backed by persister, with rooms torn
// down roomGracePeriod after becoming empty (see RoomManager).
func NewServer(persister Persister, roomGracePeriod time.Duration) *Server {
	return &Server{
		Connections: NewConnectionManager(),
		Rooms:       NewRoomManager(persister, roomGracePeriod),
	}
}

// ServeWS is an http.HandlerFunc-compatible method that upgrades the
// request to a WebSocket connection and runs it to completion.
func (s *Server) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	conn.SetReadDeadline(time.Now().Add(joinDeadline))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return
	}

	env, err := protocol.Decode(raw)
	if err != nil || env.Type != protocol.TypeJoinDocument {
		if errEnv, buildErr := buildErrorEnvelope(errFirstMessageMustBeJoin); buildErr == nil {
			if data, encErr := protocol.Encode(errEnv); encErr == nil {
				conn.WriteMessage(websocket.TextMessage, data)
			}
		}
		conn.Close()
		return
	}

	client := NewClient(env.ClientID, conn)
	s.Connections.Register(client)

	ctx := r.Context()
	room, err := s.Rooms.GetOrCreateRoom(ctx, env.DocumentID)
	if err != nil {
		log.Printf("sync: failed to join document %q: %v", env.DocumentID, err)
		s.Connections.Unregister(client)
		client.Close("failed to join document")
		return
	}
	room.Join(client)

	go client.WritePump()

	if err := room.SendInitialSync(ctx, client); err != nil {
		log.Printf("sync: sending initial_sync for document %q: %v", env.DocumentID, err)
	}

	// Blocks until the connection ends.
	client.ReadPump(ctx, func(ctx context.Context, from *Client, env protocol.Envelope) {
		HandleEnvelope(ctx, room, from, env)
	})

	room.Leave(client)
	s.Rooms.NotifyClientLeft(env.DocumentID)
	s.Connections.Unregister(client)
	client.Close("connection closed")
}

// errFirstMessageMustBeJoin is wrapped into the error envelope sent when
// a connection's first message isn't join_document.
var errFirstMessageMustBeJoin = errors.New("the first message on a new connection must be join_document")
