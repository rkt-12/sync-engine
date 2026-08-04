import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync } from "fs";
import path from "path";
import { Document } from "./document";
import { unmarshalOperation } from "./serialization";

// Loads and applies the same shared JSON test vectors that internal/crdt/vectors_test.go consumes independently on the Go side.

interface TestVectorFile {
  description: string;
  documentId: string;
  operations: unknown[];
  expected: string;
}

const testdataDir = path.resolve(import.meta.dirname, "../../../../testdata");

describe("cross-language test vectors", () => {
  const filenames = readdirSync(testdataDir).filter((f) => f.endsWith(".json"));

  it("found at least one vector file", () => {
    expect(filenames.length).toBeGreaterThan(0);
  });

  it.each(filenames)("%s", (filename) => {
    const raw = readFileSync(path.join(testdataDir, filename), "utf-8");
    const vector = JSON.parse(raw) as TestVectorFile;

    const doc = new Document(vector.documentId);
    vector.operations.forEach((rawOp, i) => {
      const op = unmarshalOperation(JSON.stringify(rawOp));
      try {
        doc.apply(op);
      } catch (err) {
        throw new Error(`${filename}: applying operation ${i + 1}: ${(err as Error).message}`);
      }
    });

    expect(doc.materialize(), `${filename}: ${vector.description}`).toBe(vector.expected);
  });
});