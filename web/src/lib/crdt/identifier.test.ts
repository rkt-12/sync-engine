import { describe, expect, it } from "vitest";
import { RootID, isGreater, isLess, isRoot } from "./identifier";

describe("identifier ordering", () => {
  it("orders by counter first", () => {
    const a = { clientId: 5, counter: 1 };
    const b = { clientId: 1, counter: 2 };
    expect(isLess(a, b)).toBe(true);
    expect(isLess(b, a)).toBe(false);
  });

  it("breaks ties on clientId when counters are equal", () => {
    const a = { clientId: 1, counter: 7 };
    const b = { clientId: 2, counter: 7 };
    expect(isLess(a, b)).toBe(true);
    expect(isLess(b, a)).toBe(false);
  });

  it("treats equal identifiers as neither less than the other", () => {
    const a = { clientId: 3, counter: 9 };
    const b = { clientId: 3, counter: 9 };
    expect(isLess(a, b)).toBe(false);
    expect(isLess(b, a)).toBe(false);
  });

  it("isGreater is the inverse of isLess", () => {
    const a = { clientId: 1, counter: 1 };
    const b = { clientId: 1, counter: 2 };
    expect(isGreater(b, a)).toBe(true);
    expect(isGreater(a, b)).toBe(false);
  });

  it("is asymmetric for every pair tested", () => {
    const pairs: [{ clientId: number; counter: number }, { clientId: number; counter: number }][] = [
      [{ clientId: 1, counter: 1 }, { clientId: 2, counter: 1 }],
      [{ clientId: 1, counter: 5 }, { clientId: 1, counter: 6 }],
      [{ clientId: 9, counter: 3 }, { clientId: 1, counter: 4 }],
    ];
    for (const [a, b] of pairs) {
      expect(isLess(a, b)).not.toBe(isLess(b, a));
    }
  });
});

describe("RootID", () => {
  it("reports isRoot true for RootID itself", () => {
    expect(isRoot(RootID)).toBe(true);
  });

  it("reports isRoot false for a non-sentinel identifier", () => {
    expect(isRoot({ clientId: 1, counter: 1 })).toBe(false);
  });
});