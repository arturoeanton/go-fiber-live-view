package crdt

import (
	"math/rand"
	"testing"
)

func TestBasicTyping(t *testing.T) {
	r := New(1)
	for i, ru := range "hola mundo" {
		r.LocalInsert(i, ru)
	}
	if r.String() != "hola mundo" {
		t.Fatalf("got %q", r.String())
	}
	r.LocalDelete(4) // delete the space
	if r.String() != "holamundo" {
		t.Fatalf("got %q", r.String())
	}
}

func TestDiffOps(t *testing.T) {
	r := New(1)
	r.DiffOps("hola mundo")
	if r.String() != "hola mundo" {
		t.Fatalf("got %q", r.String())
	}
	r.DiffOps("hola gran mundo")
	if r.String() != "hola gran mundo" {
		t.Fatalf("got %q", r.String())
	}
	r.DiffOps("chau mundo")
	if r.String() != "chau mundo" {
		t.Fatalf("got %q", r.String())
	}
	r.DiffOps("")
	if r.String() != "" {
		t.Fatalf("got %q", r.String())
	}
}

// TestConvergenceConcurrent checks that two replicas editing the same base
// concurrently converge after exchanging ops, regardless of apply order.
func TestConvergenceConcurrent(t *testing.T) {
	base := New(1)
	var seed []Op
	for i, ru := range "abc" {
		seed = append(seed, base.LocalInsert(i, ru))
	}

	a := Load(2, base.Snapshot())
	b := Load(3, base.Snapshot())

	opsA := []Op{a.LocalInsert(1, 'X'), a.LocalInsert(2, 'Y')} // a: aXYbc
	opsB := []Op{a2op(b)}                                      // b: aZbc (same spot)

	for _, op := range opsB {
		a.Apply(op)
	}
	for i := len(opsA) - 1; i >= 0; i-- { // reversed order on purpose
		b.Apply(opsA[i])
	}

	if a.String() != b.String() {
		t.Fatalf("diverged: a=%q b=%q", a.String(), b.String())
	}
}

func a2op(b *RGA) Op { return b.LocalInsert(1, 'Z') }

// TestFuzzConvergence runs random concurrent edit rounds across three
// replicas relayed through a central server replica (star topology, like
// the framework) and requires all replicas to converge every round.
func TestFuzzConvergence(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	server := New(1)
	clients := []*RGA{Load(2, nil), Load(3, nil), Load(4, nil)}

	for round := 0; round < 200; round++ {
		// each client makes 0-3 local edits and "sends" them
		var batches [][]Op
		for _, c := range clients {
			var ops []Op
			for k := 0; k < rng.Intn(4); k++ {
				if c.Len() > 0 && rng.Intn(3) == 0 {
					if op, ok := c.LocalDelete(rng.Intn(c.Len())); ok {
						ops = append(ops, op)
					}
				} else {
					pos := 0
					if c.Len() > 0 {
						pos = rng.Intn(c.Len() + 1)
					}
					ops = append(ops, c.LocalInsert(pos, rune('a'+rng.Intn(26))))
				}
			}
			batches = append(batches, ops)
		}
		// server applies batches in random order and relays to the others
		order := rng.Perm(len(batches))
		for _, bi := range order {
			for _, op := range batches[bi] {
				server.Apply(op)
			}
			for ci, c := range clients {
				if ci == bi {
					continue
				}
				for _, op := range batches[bi] {
					c.Apply(op)
				}
			}
		}
		want := server.String()
		for ci, c := range clients {
			if c.String() != want {
				t.Fatalf("round %d: client %d diverged:\nserver=%q\nclient=%q", round, ci, want, c.String())
			}
		}
	}
}

func TestPendingAnchor(t *testing.T) {
	a := New(2)
	op1 := a.LocalInsert(0, 'x')
	op2 := a.LocalInsert(1, 'y') // anchored on x

	b := New(3)
	b.Apply(op2) // arrives before his anchor: buffered
	if b.String() != "" {
		t.Fatalf("expected pending, got %q", b.String())
	}
	b.Apply(op1)
	if b.String() != "xy" {
		t.Fatalf("got %q", b.String())
	}
}
