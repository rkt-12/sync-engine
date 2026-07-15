package crdt

import "testing"

func TestIdentifierLess_ByCounter(t *testing.T) {
	a := Identifier{ClientID: 5, Counter: 1}
	b := Identifier{ClientID: 1, Counter: 2}

	if !a.Less(b) {
		t.Errorf("expected %+v < %+v (lower counter wins regardless of ClientID)", a, b)
	}
	if b.Less(a) {
		t.Errorf("expected %+v NOT < %+v", b, a)
	}
}

func TestIdentifierLess_TieBrokenByClientID(t *testing.T) {
	a := Identifier{ClientID: 1, Counter: 7}
	b := Identifier{ClientID: 2, Counter: 7}

	if !a.Less(b) {
		t.Errorf("expected %+v < %+v (same counter, lower ClientID wins)", a, b)
	}
	if b.Less(a) {
		t.Errorf("expected %+v NOT < %+v", b, a)
	}
}

func TestIdentifierLess_Equal(t *testing.T) {
	a := Identifier{ClientID: 3, Counter: 9}
	b := Identifier{ClientID: 3, Counter: 9}

	if a.Less(b) || b.Less(a) {
		t.Errorf("equal identifiers must not be Less than each other: %+v, %+v", a, b)
	}
}

func TestIdentifierGreater_IsInverseOfLess(t *testing.T) {
	a := Identifier{ClientID: 1, Counter: 1}
	b := Identifier{ClientID: 1, Counter: 2}

	if !b.Greater(a) {
		t.Errorf("expected %+v > %+v", b, a)
	}
	if a.Greater(b) {
		t.Errorf("expected %+v NOT > %+v", a, b)
	}
}

func TestIdentifierOrdering_NeverDependsOnFieldOrderOfComparison(t *testing.T) {
	// Sanity check that Less/Greater are proper strict-order inverses for a handful of pairs
	pairs := []struct{ a, b Identifier }{
		{Identifier{1, 1}, Identifier{2, 1}},
		{Identifier{1, 5}, Identifier{1, 6}},
		{Identifier{9, 3}, Identifier{1, 4}},
	}
	for _, p := range pairs {
		if p.a.Less(p.b) == p.b.Less(p.a) {
			t.Errorf("Less must be asymmetric for distinct identifiers: %+v vs %+v", p.a, p.b)
		}
	}
}

func TestRootID_IsRoot(t *testing.T) {
	if !RootID.IsRoot() {
		t.Error("RootID.IsRoot() should be true")
	}
	other := Identifier{ClientID: 1, Counter: 1}
	if other.IsRoot() {
		t.Error("a non-sentinel identifier should not report IsRoot() == true")
	}
}
