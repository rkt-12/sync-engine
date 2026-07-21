package simulation

import (
	"fmt"
	"math/rand"
)

// Config controls one randomized convergence run.
type Config struct {
	Seed                 int64
	NumClients           int
	NumOperations        int     // total local edits generated across all clients combined
	DeleteProbability    float64 // chance a generated event is a delete rather than an insert
	DuplicateProbability float64 // chance a delivered operation is redelivered a second time
	Alphabet             string
	DocumentID           string
}

// Result carries the outcome of one Run
type Result struct {
	Config      Config
	FinalStates []string
	Converged   bool
}

// Run executes one seeded, deterministic simulation: cfg.NumClients independent replicas each generate local edits, and
// every generated operation is then delivered to every other client in a randomized but causally valid order.
func Run(cfg Config) Result {
	rng := rand.New(rand.NewSource(cfg.Seed))

	clients := make([]*Client, cfg.NumClients)
	for i := range clients {
		clients[i] = NewClient(uint64(i+1), cfg.DocumentID)
	}

	allOps := make([]recordedOp, 0, cfg.NumOperations)
	for i := 0; i < cfg.NumOperations; i++ {
		c := clients[rng.Intn(len(clients))]

		var op recordedOp
		if rng.Float64() < cfg.DeleteProbability {
			if delOp, ok := c.GenerateDelete(rng); ok {
				op = recordedOp{op: delOp, generator: c.ID}
			} else {
				op = recordedOp{op: c.GenerateInsert(rng, cfg.Alphabet), generator: c.ID}
			}
		} else {
			op = recordedOp{op: c.GenerateInsert(rng, cfg.Alphabet), generator: c.ID}
		}

		allOps = append(allOps, op)
	}

	for _, target := range clients {
		for _, op := range deliveryPlan(rng, target.ID, allOps, cfg.DuplicateProbability) {
			if err := target.Deliver(op); err != nil {
				panic(fmt.Sprintf(
					"simulation: seed=%d clients=%d operations=%d: client %d failed to deliver operation (id=%v): %v",
					cfg.Seed, cfg.NumClients, cfg.NumOperations, target.ID, op.ID(), err))
			}
		}
	}

	states := make([]string, len(clients))
	converged := true
	for i, c := range clients {
		states[i] = c.Replica.Materialize()
		if i > 0 && states[i] != states[0] {
			converged = false
		}
	}

	return Result{Config: cfg, FinalStates: states, Converged: converged}
}
