package simulation

import (
	"reflect"
	"testing"
)

func runAndCheckConvergence(t *testing.T, cfg Config) Result {
	t.Helper()
	result := Run(cfg)
	if !result.Converged {
		t.Fatalf("convergence failure: seed=%d clients=%d operations=%d deleteProbability=%v duplicateProbability=%v\nfinal states: %v",
			cfg.Seed, cfg.NumClients, cfg.NumOperations, cfg.DeleteProbability, cfg.DuplicateProbability, result.FinalStates)
	}
	return result
}

func TestRandomizedConvergence(t *testing.T) {
	const numSeeds = 2000

	for seed := int64(0); seed < numSeeds; seed++ {
		cfg := Config{
			Seed:                 seed,
			NumClients:           5,
			NumOperations:        60,
			DeleteProbability:    0.3,
			DuplicateProbability: 0.2,
			Alphabet:             "abcdefghij",
			DocumentID:           "sim-doc",
		}
		runAndCheckConvergence(t, cfg)
	}
}

func TestRandomizedConvergence_LargerScale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping larger-scale simulation in -short mode")
	}

	const numSeeds = 50
	for seed := int64(100000); seed < 100000+numSeeds; seed++ {
		cfg := Config{
			Seed:                 seed,
			NumClients:           10,
			NumOperations:        1000,
			DeleteProbability:    0.35,
			DuplicateProbability: 0.15,
			Alphabet:             "abcdefghijklmnopqrstuvwxyz",
			DocumentID:           "sim-doc-large",
		}
		runAndCheckConvergence(t, cfg)
	}
}

func TestRandomizedConvergence_NoDuplicates(t *testing.T) {
	for seed := int64(0); seed < 500; seed++ {
		cfg := Config{
			Seed:                 seed,
			NumClients:           6,
			NumOperations:        80,
			DeleteProbability:    0.3,
			DuplicateProbability: 0,
			Alphabet:             "abc",
			DocumentID:           "sim-doc",
		}
		runAndCheckConvergence(t, cfg)
	}
}

func TestRandomizedConvergence_NoDeletes(t *testing.T) {
	for seed := int64(0); seed < 500; seed++ {
		cfg := Config{
			Seed:                 seed,
			NumClients:           6,
			NumOperations:        80,
			DeleteProbability:    0,
			DuplicateProbability: 0.3,
			Alphabet:             "abc",
			DocumentID:           "sim-doc",
		}
		runAndCheckConvergence(t, cfg)
	}
}

func TestRandomizedConvergence_HighContention(t *testing.T) {
	for seed := int64(0); seed < 500; seed++ {
		cfg := Config{
			Seed:                 seed,
			NumClients:           15,
			NumOperations:        40,
			DeleteProbability:    0.5,
			DuplicateProbability: 0.4,
			Alphabet:             "xy",
			DocumentID:           "sim-doc",
		}
		runAndCheckConvergence(t, cfg)
	}
}

func TestRun_DeterministicForSameSeed(t *testing.T) {
	cfg := Config{
		Seed:                 42,
		NumClients:           4,
		NumOperations:        150,
		DeleteProbability:    0.3,
		DuplicateProbability: 0.25,
		Alphabet:             "abcdef",
		DocumentID:           "doc",
	}

	r1 := Run(cfg)
	r2 := Run(cfg)

	if !reflect.DeepEqual(r1.FinalStates, r2.FinalStates) {
		t.Fatalf("Run(cfg) was not deterministic for the same seed:\n  run1=%v\n  run2=%v",
			r1.FinalStates, r2.FinalStates)
	}
}

func TestRun_ProducesNonTrivialDocument(t *testing.T) {
	cfg := Config{
		Seed:                 7,
		NumClients:           5,
		NumOperations:        60,
		DeleteProbability:    0.2,
		DuplicateProbability: 0.2,
		Alphabet:             "abcdefghij",
		DocumentID:           "doc",
	}

	result := runAndCheckConvergence(t, cfg)
	if len(result.FinalStates[0]) == 0 {
		t.Error("expected a non-empty converged document for this scenario, got empty string")
	}
}
