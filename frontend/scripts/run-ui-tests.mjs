#!/usr/bin/env node
/**
 * Run all frontend UI helper / unit scripts (no headed browser).
 * Exit non-zero if any script fails.
 */
import { spawnSync } from 'node:child_process'
import { readdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const dir = dirname(fileURLToPath(import.meta.url))
const frontendRoot = join(dir, '..')
const scripts = readdirSync(dir)
  .filter((name) => name.startsWith('test-') && name.endsWith('.mjs'))
  .sort()

if (!scripts.length) {
  console.error('no test-*.mjs scripts found')
  process.exit(1)
}

/** scripts that import Vue SFCs or extensionless .ts re-exports */
const needsResolveHook = new Set(['test-slides-render.mjs', 'test-ms-office-convert.mjs'])

let failed = 0
for (const name of scripts) {
  const path = join(dir, name)
  const args = needsResolveHook.has(name)
    ? ['--import', './scripts/register-ts-resolve.mjs', '--experimental-strip-types', path]
    : ['--experimental-strip-types', path]
  console.log(`\n==> ${name}`)
  const r = spawnSync(process.execPath, args, {
    cwd: frontendRoot,
    stdio: 'inherit',
  })
  if (r.status !== 0) {
    failed += 1
    console.error(`FAIL ${name} (exit ${r.status ?? 'signal'})`)
  }
}

if (failed) {
  console.error(`\n${failed}/${scripts.length} frontend UI test script(s) failed`)
  process.exit(1)
}
console.log(`\nOK ${scripts.length} frontend UI test script(s)`)
