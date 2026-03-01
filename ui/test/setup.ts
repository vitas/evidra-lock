import "@testing-library/jest-dom/vitest";

// Provide localStorage and sessionStorage for jsdom
const createStorage = (): Storage => {
  let store: Record<string, string> = {};
  return {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => {
      store[key] = String(value);
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      store = {};
    },
    get length() {
      return Object.keys(store).length;
    },
    key: (index: number) => Object.keys(store)[index] ?? null,
  };
};

Object.defineProperty(globalThis, "localStorage", { value: createStorage() });
Object.defineProperty(globalThis, "sessionStorage", { value: createStorage() });

// jsdom does not provide IntersectionObserver; mock it for component effects.
class MockIntersectionObserver implements IntersectionObserver {
  readonly root: Element | Document | null = null;
  readonly rootMargin: string;
  readonly thresholds: ReadonlyArray<number>;

  constructor(
    _callback: IntersectionObserverCallback,
    options?: IntersectionObserverInit,
  ) {
    this.root = options?.root ?? null;
    this.rootMargin = options?.rootMargin ?? "0px";
    const threshold = options?.threshold ?? 0;
    this.thresholds = Array.isArray(threshold) ? threshold : [threshold];
  }

  disconnect(): void {}

  observe(_target: Element): void {}

  takeRecords(): IntersectionObserverEntry[] {
    return [];
  }

  unobserve(_target: Element): void {}
}

Object.defineProperty(globalThis, "IntersectionObserver", {
  value: MockIntersectionObserver,
  writable: true,
});
