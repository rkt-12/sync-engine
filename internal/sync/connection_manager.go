package sync

import "sync"

type closable interface {
	Close(reason string)
}

// ConnectionManager tracks every currently live WebSocket connection, independent of which document it's joined to.
type ConnectionManager struct {
	mu      sync.Mutex
	clients map[closable]struct{}
}

// NewConnectionManager constructs an empty ConnectionManager.
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{clients: make(map[closable]struct{})}
}

// Register adds c to the set of live connections.
func (m *ConnectionManager) Register(c closable) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[c] = struct{}{}
}

// Unregister removes c from the set of live connections.
func (m *ConnectionManager) Unregister(c closable) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, c)
}

// Count returns the number of currently registered live connections.
func (m *ConnectionManager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.clients)
}

// Shutdown closes every currently registered connection with the given reason.
func (m *ConnectionManager) Shutdown(reason string) {
	m.mu.Lock()
	snapshot := make([]closable, 0, len(m.clients))
	for c := range m.clients {
		snapshot = append(snapshot, c)
	}
	m.mu.Unlock()

	for _, c := range snapshot {
		c.Close(reason)
	}
}
