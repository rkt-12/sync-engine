import { describe, expect, it } from "vitest";
import { LogicalClock } from "./clock";

describe("LogicalClock", () => {
  it("increments on next()", () => {
    const c = new LogicalClock();
    expect(c.next()).toBe(1);
    expect(c.next()).toBe(2);
    expect(c.current()).toBe(2);
  });

  it("advances to the observed value if higher", () => {
    const c = new LogicalClock();
    c.next(); // 1
    c.observe(10);
    expect(c.current()).toBe(10);
  });

  it("never moves backward on observe", () => {
    const c = new LogicalClock();
    c.observe(10);
    c.observe(3);
    expect(c.current()).toBe(10);
  });

  it("continues forward correctly after observe", () => {
    const c = new LogicalClock();
    c.observe(100);
    expect(c.next()).toBe(101);
  });
});