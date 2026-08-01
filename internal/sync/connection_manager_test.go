package sync

import "testing"

type fakeClosable struct {
	id     string
	closed bool
	reason string
}

func (f *fakeClosable) Close(reason string) {
	f.closed = true
	f.reason = reason
}

func TestConnectionManager_RegisterUnregisterCount(t *testing.T) {
	mgr := NewConnectionManager()
	c1 := &fakeClosable{id: "c1"}
	c2 := &fakeClosable{id: "c2"}

	mgr.Register(c1)
	mgr.Register(c2)
	if got := mgr.Count(); got != 2 {
		t.Fatalf("Count() = %d, want 2", got)
	}

	mgr.Unregister(c1)
	if got := mgr.Count(); got != 1 {
		t.Fatalf("Count() = %d, want 1", got)
	}
}

func TestConnectionManager_Shutdown_ClosesAll(t *testing.T) {
	mgr := NewConnectionManager()
	c1 := &fakeClosable{id: "c1"}
	c2 := &fakeClosable{id: "c2"}
	c3 := &fakeClosable{id: "c3"}
	mgr.Register(c1)
	mgr.Register(c2)
	mgr.Register(c3)

	mgr.Shutdown("server shutting down")

	for _, c := range []*fakeClosable{c1, c2, c3} {
		if !c.closed {
			t.Errorf("expected %s to be closed after Shutdown", c.id)
		}
		if c.reason != "server shutting down" {
			t.Errorf("expected close reason %q, got %q", "server shutting down", c.reason)
		}
	}
}

func TestConnectionManager_Shutdown_ReentrantUnregisterDoesNotDeadlock(t *testing.T) {
	mgr := NewConnectionManager()
	// A closable whose Close call itself calls back into the manager --
	// simulating a real Client whose cleanup path unregisters itself.
	var self *reentrantClosable
	self = &reentrantClosable{mgr: mgr}
	mgr.Register(self)

	done := make(chan struct{})
	go func() {
		mgr.Shutdown("test")
		close(done)
	}()

	select {
	case <-done:
	default:
	}
	// If Shutdown deadlocks, this test will hang and be killed by the
	// test timeout rather than failing cleanly -- that's an acceptable
	// signal for this kind of test.
	<-done
	_ = self
}

type reentrantClosable struct {
	mgr *ConnectionManager
}

func (r *reentrantClosable) Close(reason string) {
	r.mgr.Unregister(r)
}
