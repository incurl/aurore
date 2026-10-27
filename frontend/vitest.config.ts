import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    // The zoom tests need a DOM to assert against.
    environment: 'jsdom',
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
  },
});
