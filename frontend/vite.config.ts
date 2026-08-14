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
