import path from 'node:path'
import { pathToFileURL } from 'node:url'
import fs from 'node:fs'

const srcRoot = path.resolve(import.meta.dirname, '../src')

function exists(p) {
  try {
    return fs.existsSync(p)
  } catch {
    return false
  }
}

export async function resolve(specifier, context, nextResolve) {
  if (specifier.startsWith('@/')) {
    let target = path.join(srcRoot, specifier.slice(2))
    if (!exists(target) && exists(target + '.ts')) target += '.ts'
    if (!exists(target) && exists(path.join(target, 'index.ts'))) {
      target = path.join(target, 'index.ts')
    }
    return nextResolve(pathToFileURL(target).href, context)
  }

  if (specifier.startsWith('.') && !specifier.endsWith('.ts') && !specifier.endsWith('.js') && !specifier.endsWith('.mjs') && !specifier.endsWith('.json')) {
    const parent = context.parentURL ? new URL(context.parentURL) : null
    if (parent?.pathname) {
      const candidate = path.resolve(path.dirname(parent.pathname), specifier + '.ts')
      if (exists(candidate)) {
        return nextResolve(pathToFileURL(candidate).href, context)
      }
    }
  }

  return nextResolve(specifier, context)
}
