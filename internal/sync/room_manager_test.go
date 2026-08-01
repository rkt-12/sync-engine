package sync

import (
	"context"
	"testing"
	"time"
)

func TestRoomManager_GetOrCreateRoom_ReturnsSameRoomOnSecondCall(t *testing.T) {
	mgr := NewRoomManager(newFakePersister(), time.Minute)
	ctx := context.Background()

	room1, err := mgr.GetOrCreateRoom(ctx, "doc-1")
	if err != nil {
		t.Fatalf("first GetOrCreateRoom failed: %v", err)
	}
	room2, err := mgr.GetOrCreateRoom(ctx, "doc-1")
	if err != nil {
		t.Fatalf("second GetOrCreateRoom failed: %v", err)
	}

	if room1 != room2 {
		t.Error("expected GetOrCreateRoom to return the same *DocumentRoom for the same documentID")
	}
	if mgr.RoomCount() != 1 {
		t.Errorf("RoomCount() = %d, want 1", mgr.RoomCount())
	}
}

func TestRoomManager_DifferentDocuments_GetDifferentRooms(t *testing.T) {
	mgr := NewRoomManager(newFakePersister(), time.Minute)
	ctx := context.Background()

	room1, err := mgr.GetOrCreateRoom(ctx, "doc-1")
	if err != nil {
		t.Fatalf("GetOrCreateRoom(doc-1) failed: %v", err)
	}
	room2, err := mgr.GetOrCreateRoom(ctx, "doc-2")
	if err != nil {
		t.Fatalf("GetOrCreateRoom(doc-2) failed: %v", err)
	}
	if room1 == room2 {
		t.Error("expected different documents to get different rooms")
	}
	if mgr.RoomCount() != 2 {
		t.Errorf("RoomCount() = %d, want 2", mgr.RoomCount())
	}
}

func TestRoomManager_TeardownAfterGracePeriod_WhenEmpty(t *testing.T) {
	const grace = 20 * time.Millisecond
	mgr := NewRoomManager(newFakePersister(), grace)
	ctx := context.Background()

	room, err := mgr.GetOrCreateRoom(ctx, "doc-1")
	if err != nil {
		t.Fatalf("GetOrCreateRoom failed: %v", err)
	}
	client := newFakeRoomClient("client-1")
	room.Join(client)
	room.Leave(client)

	mgr.NotifyClientLeft("doc-1")

	if mgr.RoomCount() != 1 {
		t.Fatalf("expected the room to still exist immediately after NotifyClientLeft (grace period not yet elapsed), RoomCount() = %d", mgr.RoomCount())
	}

	deadline := time.Now().Add(2 * time.Second)
	for mgr.RoomCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if mgr.RoomCount() != 0 {
		t.Errorf("expected the room to be torn down after the grace period elapsed, RoomCount() = %d", mgr.RoomCount())
	}
}

func TestRoomManager_TeardownCancelled_IfRejoinedBeforeGracePeriod(t *testing.T) {
	const grace = 200 * time.Millisecond
	mgr := NewRoomManager(newFakePersister(), grace)
	ctx := context.Background()

	room, err := mgr.GetOrCreateRoom(ctx, "doc-1")
	if err != nil {
		t.Fatalf("GetOrCreateRoom failed: %v", err)
	}
	client := newFakeRoomClient("client-1")
	room.Join(client)
	room.Leave(client)
	mgr.NotifyClientLeft("doc-1")

	// Rejoin well before the grace period elapses.
	time.Sleep(grace / 4)
	sameRoom, err := mgr.GetOrCreateRoom(ctx, "doc-1")
	if err != nil {
		t.Fatalf("second GetOrCreateRoom failed: %v", err)
	}
	if sameRoom != room {
		t.Error("expected the same room instance to be reused when rejoined before teardown")
	}

	// Wait past the original grace period -- the room must NOT have been
	// torn down, since the pending teardown should have been cancelled.
	time.Sleep(grace)
	if mgr.RoomCount() != 1 {
		t.Errorf("expected the room to survive after being rejoined before its scheduled teardown, RoomCount() = %d", mgr.RoomCount())
	}
}

func TestRoomManager_NotifyClientLeft_NoOpIfRoomStillHasClients(t *testing.T) {
	const grace = 20 * time.Millisecond
	mgr := NewRoomManager(newFakePersister(), grace)
	ctx := context.Background()

	room, err := mgr.GetOrCreateRoom(ctx, "doc-1")
	if err != nil {
		t.Fatalf("GetOrCreateRoom failed: %v", err)
	}
	// Two clients join; only one leaves -- room is not empty.
	c1 := newFakeRoomClient("client-1")
	c2 := newFakeRoomClient("client-2")
	room.Join(c1)
	room.Join(c2)
	room.Leave(c1)

	mgr.NotifyClientLeft("doc-1")

	time.Sleep(grace * 3)
	if mgr.RoomCount() != 1 {
		t.Errorf("expected the room to survive since it still has a client, RoomCount() = %d", mgr.RoomCount())
	}
}
