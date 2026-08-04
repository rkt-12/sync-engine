// TypeScript counterpart to internal/crdt/clock.go's LogicalClock: a Lamport clock, one monotonically increasing counter per client.

export class LogicalClock {
  private counter = 0;

  next(): number {
    this.counter += 1;
    return this.counter;
  }

  observe(remote: number): void {
    if (remote > this.counter) {
      this.counter = remote;
    }
  }

  current(): number {
    return this.counter;
  }
}