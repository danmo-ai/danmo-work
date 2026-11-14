/**
 * Runtime Vue SFC preview.
 *
 * The Stage can render a project `.vue` file live: parse → compileScript /
 * compileTemplate → evaluate the generated module from a Blob URL. Compilation
 * happens in the browser so agents can write a component and see it without a
 * build step.
 *
 * Only `vue` imports are supported. Anything else (relative imports, aliases,
 * TS-only syntax the compiler can't strip) fails loudly and the surface falls
 * back to the source view.
 */

/** `import ... from "vue"` — the only bare specifier resolved at runtime. */
const VUE_IMPORT_RE =
  /import\s+(?:\*\s*as\s+([\w$]+)|\{([\s\S]*?)\})\s*from\s*['"]vue['"]\s*;?/g

export const SFC_VUE_GLOBAL = '__DANMO_SFC_VUE__'

interface Collected {
  /** local binding → exported name */
  named: Map<string, string>
  namespaces: string[]
  rest: string
}

function collectVueImports(code: string): Collected {
  const named = new Map<string, string>()
  const namespaces: string[] = []
  const rest = code.replace(VUE_IMPORT_RE, (_full, ns?: string, members?: string) => {
    if (ns) {
      namespaces.push(ns)
      return ''
    }
    for (const raw of (members ?? '').split(',')) {
      const item = raw.trim()
      if (!item) continue
      const alias = item.match(/^([\w$]+)\s+as\s+([\w$]+)$/)
      if (alias) named.set(alias[2], alias[1])
      else named.set(item, item)
    }
    return ''
  })
  return { named, namespaces, rest }
}

/** Turn collected bindings into one destructure off the injected namespace. */
function bindingHeader(collected: Collected[], globalName: string): string[] {
  const named = new Map<string, string>()
  const namespaces = new Set<string>()
  for (const c of collected) {
    for (const [local, imported] of c.named) named.set(local, imported)
    for (const ns of c.namespaces) namespaces.add(ns)
  }
  const lines: string[] = []
  if (named.size) {
    const pairs = [...named].map(([local, imported]) =>
      local === imported ? local : `${imported}: ${local}`,
    )
    lines.push(`const { ${pairs.join(', ')} } = ${globalName};`)
  }
  for (const ns of namespaces) lines.push(`const ${ns} = ${globalName};`)
  return lines
}

export interface SfcModuleParts {
  /** compileScript output; keeps `export default` when `inline` is true. */
  script?: string
  /** compileTemplate output (exports `render`). */
  template?: string
  /** True when compileScript already inlined the template into the default export. */
  inline: boolean
}

/**
 * Assemble the evaluable module for a compiled SFC.
 *
 * Split mode rewrites `export default` into a const so the separately compiled
 * `render` can be attached to it; Vue's two generated chunks both import from
 * `vue`, so bindings are merged into one destructure to avoid redeclarations.
 */
export function assembleSfcModule(parts: SfcModuleParts, globalName = SFC_VUE_GLOBAL): string {
  const script = collectVueImports(parts.script ?? '')
  const template = collectVueImports(parts.template ?? '')
  const header = bindingHeader([script, template], globalName)

  if (parts.inline) {
    return [...header, script.rest.trim()].filter(Boolean).join('\n')
  }

  const scriptBody = script.rest.trim()
    ? script.rest.replace(/export\s+default\s+/, 'const __sfc_main = ')
    : 'const __sfc_main = {};'
  const templateBody = template.rest.replace(/^[ \t]*export\s+/gm, '')
  return [
    ...header,
    scriptBody,
    templateBody.trim(),
    'export default Object.assign(__sfc_main, { render });',
  ]
    .filter(Boolean)
    .join('\n')
}

type SfcCompiler = typeof import('vue/compiler-sfc')

/**
 * `lang="ts"` survives compileScript as TypeScript, so the generated module
 * needs a type-strip pass before it can be evaluated. Loaded lazily — only SFCs
 * that actually use TypeScript pay for the compiler.
 */
async function stripTypes(code: string): Promise<string> {
  const ts = (await import('typescript')) as typeof import('typescript')
  const out = ts.transpileModule(code, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2020,
      module: ts.ModuleKind.ESNext,
      sourceMap: false,
    },
    reportDiagnostics: false,
  })
  return out.outputText
}

/** Compile SFC source into the module source we can evaluate from a Blob URL. */
export async function compileSfcToModule(source: string, filename: string): Promise<string> {
  let sfc: SfcCompiler
  try {
    // Browser build of the SFC compiler; split into its own chunk.
    sfc = (await import('vue/compiler-sfc')) as unknown as SfcCompiler
  } catch (e) {
    throw new Error(`Vue SFC compiler unavailable: ${(e as Error).message}`)
  }

  const id = `sfc-${Math.random().toString(36).slice(2, 10)}`
  const { descriptor, errors } = sfc.parse(source, { filename })
  if (errors.length) {
    throw new Error(errors.map((e) => (e as Error).message).join('\n'))
  }

  const isTs = [descriptor.script?.lang, descriptor.scriptSetup?.lang].some(
    (lang) => lang === 'ts' || lang === 'tsx',
  )

  const finish = async (module: string) => (isTs ? stripTypes(module) : module)

  const hasScriptSetup = Boolean(descriptor.scriptSetup)
  if (descriptor.script || descriptor.scriptSetup) {
    const compiled = sfc.compileScript(descriptor, { id, inlineTemplate: hasScriptSetup })
    if (!hasScriptSetup && descriptor.template) {
      const tpl = sfc.compileTemplate({
        id,
        filename,
        source: descriptor.template.content,
        compilerOptions: { bindingMetadata: compiled.bindings },
      })
      if (tpl.errors.length) throw new Error(tpl.errors.map(String).join('\n'))
      return finish(assembleSfcModule({ script: compiled.content, template: tpl.code, inline: false }))
    }
    if (hasScriptSetup) {
      return finish(assembleSfcModule({ script: compiled.content, inline: true }))
    }
    // <script> only, no template.
    return finish(assembleSfcModule({ script: compiled.content, inline: false }))
  }

  if (!descriptor.template) throw new Error('Component has neither <template> nor <script>.')

  const tpl = sfc.compileTemplate({ id, filename, source: descriptor.template.content })
  if (tpl.errors.length) throw new Error(tpl.errors.map(String).join('\n'))
  return finish(assembleSfcModule({ template: tpl.code, inline: false }))
}

/** Compile + evaluate, returning the component definition. */
export async function loadSfcComponent(source: string, filename: string): Promise<unknown> {
  const [vue, moduleSource] = await Promise.all([
    import('vue'),
    compileSfcToModule(source, filename),
  ])
  ;(globalThis as Record<string, unknown>)[SFC_VUE_GLOBAL] = vue
  const url = URL.createObjectURL(new Blob([moduleSource], { type: 'text/javascript' }))
  try {
    const mod = (await import(/* @vite-ignore */ url)) as { default?: unknown }
    if (!mod.default) throw new Error('Compiled component has no default export.')
    return mod.default
  } finally {
    URL.revokeObjectURL(url)
  }
}
