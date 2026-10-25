// @ts-check
/** @type {import('@stryker-mutator/api/core').PartialStrykerOptions} */
export default {
  packageManager: 'npm',
  testRunner: 'vitest',
  // Vitest 5 needs @stryker-mutator/vitest-runner >= 10.1.0: Vitest 5 changed
  // testNamePattern's separator to ' > ', and older runners still joined it
  // with a space, so the per-mutant test filter matched nothing — every
  // covered mutant ran 0 tests and was reported Survived (score ~4%). Fixed
  // in https://github.com/stryker-mutator/stryker-js/issues/6210; do not
  // downgrade the runner below 10.1.0 while on vitest 5.
  vitest: {
    configFile: 'vite.config.ts',
  },
  mutate: [
    'src/**/*.{ts,tsx}',
    '!src/main.tsx',
    '!src/vite-env.d.ts',
    '!src/**/*.test.{ts,tsx}',
    '!src/test/**',
    // Vendored shadcn/ui code (copied in via `npx shadcn add`, not
    // hand-authored business logic) — excluded the same way it's excluded
    // from the coverage gate (vite.config.ts) and Go's Mockery mocks
    // (ADR-003, .gremlins.yaml).
    '!src/components/ui/**',
    '!src/lib/utils.ts',
  ],
  reporters: ['progress', 'clear-text', 'html', 'json'],
  htmlReporter: { fileName: 'reports/mutation/index.html' },
  jsonReporter: { fileName: 'reports/mutation/mutation.json' },
  thresholds: { high: 90, low: 80, break: 80 },
  tempDirName: '.stryker-tmp',
  // Skip mutants unaffected by changes since the last run. CI restores/saves
  // reports/stryker-incremental.json across `push` runs on main/develop (see
  // ci.yml); locally it just makes repeated `npm run mutation` runs faster.
  incremental: true,
}
