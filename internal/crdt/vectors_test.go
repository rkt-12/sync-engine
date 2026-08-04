package crdt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// testVectorFile mirrors the shared JSON test vector format documented
// in docs/crdt-specification.md ("Cross-Language Test Vectors") and
// consumed independently by both this package and web/src/lib/crdt.
// Operations are stored as raw JSON so each one can be fed directly to
// UnmarshalOperation -- the exact function real operations decode
// through, not a parallel test-only parser.
type testVectorFile struct {
	Description string            `json:"description"`
	DocumentID  string            `json:"documentId"`
	Operations  []json.RawMessage `json:"operations"`
	Expected    string            `json:"expected"`
}

func TestCrossLanguageVectors(t *testing.T) {
	dir := "../../testdata"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading testdata directory: %v", err)
	}

	var found int
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		found++
		filename := entry.Name()

		t.Run(filename, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, filename))
			if err != nil {
				t.Fatalf("reading %s: %v", filename, err)
			}

			var vector testVectorFile
			if err := json.Unmarshal(data, &vector); err != nil {
				t.Fatalf("parsing %s: %v", filename, err)
			}

			doc := NewDocument(vector.DocumentID)
			for i, raw := range vector.Operations {
				op, err := UnmarshalOperation(raw)
				if err != nil {
					t.Fatalf("%s: decoding operation %d: %v", filename, i+1, err)
				}
				if err := doc.Apply(op); err != nil {
					t.Fatalf("%s: applying operation %d: %v", filename, i+1, err)
				}
			}

			if got := doc.Materialize(); got != vector.Expected {
				t.Errorf("%s: Materialize() = %q, want %q\n  description: %s",
					filename, got, vector.Expected, vector.Description)
			}
		})
	}

	if found == 0 {
		t.Fatal("no test vector files found in testdata -- did the directory move?")
	}
	t.Logf("verified %d cross-language test vectors", found)
}
