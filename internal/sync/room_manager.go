package sync

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RoomManager maps documentID -> *DocumentRoom, creating rooms lazily on
// first join and tearing them down some time after the last client leaves.
type RoomManager struct {
	persister   Persister
	gracePeriod time.Duration

	mu      sync.Mutex
	rooms   map[string]*DocumentRoom
	closing map[string]*time.Timer
}

func NewRoomManager(persister Persister, gracePeriod time.Duration) *RoomManager {
	return &RoomManager{
		persister:   persister,
		gracePeriod: gracePeriod,
		rooms:       make(map[string]*DocumentRoom),
		closing:     make(map[string]*time.Timer),
	}
}

// GetOrCreateRoom returns the existing room for documentID, or
// constructs a new one if none exists yet.
func (m *RoomManager) GetOrCreateRoom(ctx context.Context, documentID string) (*DocumentRoom, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if timer, pending := m.closing[documentID]; pending {
		timer.Stop()
		delete(m.closing, documentID)
	}

	if room, exists := m.rooms[documentID]; exists {
		return room, nil
	}

	room, err := NewDocumentRoom(ctx, documentID, m.persister)
	if err != nil {
		return nil, fmt.Errorf("room manager: creating room for document %q: %w", documentID, err)
	}
	m.rooms[documentID] = room
	return room, nil
}

// NotifyClientLeft should be called after a client leaves a document's room.
func (m *RoomManager) NotifyClientLeft(documentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, exists := m.rooms[documentID]
	if !exists || !room.IsEmpty() {
		return
	}
	if _, alreadyPending := m.closing[documentID]; alreadyPending {
		return
	}

	m.closing[documentID] = time.AfterFunc(m.gracePeriod, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if room, exists := m.rooms[documentID]; exists && room.IsEmpty() {
			delete(m.rooms, documentID)
		}
		delete(m.closing, documentID)
	})
}

// RoomCount returns the number of currently active rooms
func (m *RoomManager) RoomCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.rooms)
}
