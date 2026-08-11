import { writeFileSync } from 'node:fs'

// node_modules is regenerated on every install and is gitignored, so this
// firewall file can't be checked into git directly — it's rewritten here
// instead. See its own header comment for why it needs to exist at all.
writeFileSync(
  'node_modules/go.mod',
  [
    '// This module is a firewall, not a real Go package: some npm packages',
    '// (e.g. "flatted") ship stray .go source files inside their published',
    '// tarball. Without this file, `go build ./...`/`go test ./...` from the',
    '// repo root would try to compile them, since node_modules is otherwise',
    "// just another directory Go's package traversal walks into.",
    '//',
    '// Regenerated on every `npm install`/`npm ci` by',
    '// frontend/scripts/postinstall.mjs — do not edit by hand.',
    'module nodejs-firewall',
    '',
    'go 1.26',
    '',
  ].join('\n'),
)
