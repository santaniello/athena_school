/// <reference types="vitest/config" />
import path from 'node:path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
  test: {
    environment: 'jsdom',
    globals: false,
    clearMocks: true,
    setupFiles: ['./src/test/setup.ts'],
    // Vitest's 5s default was never sized for this suite. App's boot splash
    // deliberately holds for 1.4s (App.tsx's MIN_SPLASH_MS), so every
    // end-to-end test pays that before it can assert anything, and the
    // onboarding flow adds a dozen real user interactions across three
    // Radix Selects on top — ~2.6s on an idle machine. Running the full
    // suite in parallel puts several of those workers in contention, and
    // the two onboarding tests would intermittently cross 5s and fail on a
    // timeout rather than on anything being wrong. This is a ceiling for
    // detecting a genuinely stuck test, not a target, so it is set well
    // clear of the slowest legitimate one.
    testTimeout: 20000,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov', 'html'],
      thresholds: {
        lines: 80,
        functions: 80,
        branches: 80,
        statements: 80,
      },
      // src/components/ui is the shadcn/ui vendored component set (copied in via
      // `npx shadcn add`, not hand-authored business logic) — excluded from the
      // gate the same way Go's Mockery-generated mocks are (see ADR-003).
      exclude: ['src/main.tsx', 'src/vite-env.d.ts', 'wailsjs/**', 'src/components/ui/**'],
    },
  },
})
