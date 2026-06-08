import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js/lib/core'

// ── Lazy-loaded heavy dependencies ──
// Dynamic imports are used here because these modules are 300KB–1MB+ and only
// needed when specific content types appear (LaTeX formulas, syntax-highlighted
// code blocks). Static imports would force them into the initial bundle and
// block application startup. See: ts-no-dynamic-import exception for
// "platform-specific modules that do not exist everywhere" — these are
// content-type-specific modules that may never be needed.

let katexPromise: Promise<typeof import('katex')> | null = null
let katexModule: typeof import('katex') | null = null

function loadKatex() {
  if (!katexPromise) katexPromise = import('katex')
  return katexPromise
}

/** Callback invoked when a lazy dependency loads and a re-render may be needed. */
let onLazyLoadCallback: (() => void) | null = null

/** Register a callback to trigger re-render after lazy deps load (called once). */
export function onLazyLoad(cb: () => void) {
  onLazyLoadCallback = cb
}

// Track whether KaTeX CSS has been injected
let katexCssInjected = false

function ensureKatexCss() {
  if (katexCssInjected) return
  katexCssInjected = true
  const link = document.createElement('link')
  link.rel = 'stylesheet'
  link.href = 'katex/dist/katex.min.css'
  document.head.appendChild(link)
}

// Eagerly start loading KaTeX; caller triggers re-render when ready.
export function prefetchKatex() {
  if (katexModule) return
  loadKatex().then(mod => {
    katexModule = mod
    ensureKatexCss()
    onLazyLoadCallback?.()
  })
}

// Register mermaid language (no syntax highlighting needed, just prevents errors)
hljs.registerLanguage('mermaid', () => ({
  keywords: '',
  contains: [],
}))

// ── Lazy highlight.js language registration ──
// Language definitions are loaded on first use via dynamic import. The map
// stores pending promises so concurrent calls for the same language share one
// load. This avoids bundling 35 language files (~150KB) into the initial chunk.

import type {LanguageFn} from 'highlight.js'

type LangModule = { default: LanguageFn }

const languageLoaders: Record<string, () => Promise<LangModule>> = {
  javascript: () => import('highlight.js/lib/languages/javascript'),
  typescript: () => import('highlight.js/lib/languages/typescript'),
  python: () => import('highlight.js/lib/languages/python'),
  go: () => import('highlight.js/lib/languages/go'),
  java: () => import('highlight.js/lib/languages/java'),
  c: () => import('highlight.js/lib/languages/c'),
  cpp: () => import('highlight.js/lib/languages/cpp'),
  csharp: () => import('highlight.js/lib/languages/csharp'),
  rust: () => import('highlight.js/lib/languages/rust'),
  ruby: () => import('highlight.js/lib/languages/ruby'),
  php: () => import('highlight.js/lib/languages/php'),
  sql: () => import('highlight.js/lib/languages/sql'),
  bash: () => import('highlight.js/lib/languages/bash'),
  json: () => import('highlight.js/lib/languages/json'),
  xml: () => import('highlight.js/lib/languages/xml'),
  yaml: () => import('highlight.js/lib/languages/yaml'),
  markdown: () => import('highlight.js/lib/languages/markdown'),
  css: () => import('highlight.js/lib/languages/css'),
  diff: () => import('highlight.js/lib/languages/diff'),
  shell: () => import('highlight.js/lib/languages/shell'),
  matlab: () => import('highlight.js/lib/languages/matlab'),
  lua: () => import('highlight.js/lib/languages/lua'),
  r: () => import('highlight.js/lib/languages/r'),
  scala: () => import('highlight.js/lib/languages/scala'),
  kotlin: () => import('highlight.js/lib/languages/kotlin'),
  swift: () => import('highlight.js/lib/languages/swift'),
  perl: () => import('highlight.js/lib/languages/perl'),
  delphi: () => import('highlight.js/lib/languages/delphi'),
  objectivec: () => import('highlight.js/lib/languages/objectivec'),
  makefile: () => import('highlight.js/lib/languages/makefile'),
  nginx: () => import('highlight.js/lib/languages/nginx'),
  dockerfile: () => import('highlight.js/lib/languages/dockerfile'),
  vim: () => import('highlight.js/lib/languages/vim'),
  latex: () => import('highlight.js/lib/languages/latex'),
  ini: () => import('highlight.js/lib/languages/ini'),
}

// Map of aliases → canonical language name
const languageAliases: Record<string, string> = {
  js: 'javascript',
  ts: 'typescript',
  py: 'python',
  golang: 'go',
  'c++': 'cpp',
  cs: 'csharp',
  rb: 'ruby',
  sh: 'shell',
  html: 'xml',
  svg: 'xml',
  yml: 'yaml',
  md: 'markdown',
  'objective-c': 'objectivec',
  'objective-c++': 'objectivec',
  objc: 'objectivec',
  tex: 'latex',
  toml: 'ini',
}

// Track pending loads to avoid duplicate imports
const pendingLoads = new Map<string, Promise<void>>()

/** Try to highlight with a language; loads it lazily if not yet registered. */
function highlightWithLang(str: string, lang: string): string | null {
  if (hljs.getLanguage(lang)) {
    try {
      return hljs.highlight(str, { language: lang, ignoreIllegals: true }).value
    } catch {
      return null
    }
  }

  // Resolve alias to canonical name
  const canonical = languageAliases[lang] ?? lang
  if (languageLoaders[canonical] && !hljs.getLanguage(canonical)) {
    // Language not loaded yet — kick off async load and trigger re-render
    if (!pendingLoads.has(canonical)) {
      const loadPromise = languageLoaders[canonical]().then(mod => {
        // Register both canonical name and any aliases
        hljs.registerLanguage(canonical, mod.default)
        // Register aliases pointing to the same definition
        for (const [alias, target] of Object.entries(languageAliases)) {
          if (target === canonical && alias !== canonical) {
            hljs.registerLanguage(alias, mod.default)
          }
        }
        pendingLoads.delete(canonical)
        onLazyLoadCallback?.()
      })
      pendingLoads.set(canonical, loadPromise)
    }
  }

  return null
}

// Fix bold rendering when ** is adjacent to Unicode punctuation (e.g. Chinese quotes ""、《》)
// Per CommonMark emphasis algorithm, ** next to punctuation fails left/right-flanking checks,
// so bold is not recognized. Insert zero-width space (U+200B) as an invisible separator.
function preprocessBoldMarkdown(text: string): string {
  return text.replace(/\*\*(.+?)\*\*/g, (match, inner: string) => {
    const prefixZws = /^\p{P}/u.test(inner) ? '\u200B' : ''
    const suffixZws = /\p{P}$/u.test(inner) ? '\u200B' : ''
    if (prefixZws || suffixZws) {
      return '**' + prefixZws + inner + suffixZws + '**'
    }
    return match
  })
}

// Singleton MarkdownIt instance — shared across all MessageBubble components
export const md = new MarkdownIt({
  html: false,
  linkify: true,
  typographer: false,
})

// Override fence renderer
md.renderer.rules.fence = (tokens, idx) => {
  const token = tokens[idx]
  const str = token.content
  const lang = token.info.trim()
  const langLabel = lang || 'code'

  let highlighted: string
  if (lang) {
    const result = highlightWithLang(str, lang)
    highlighted = result ?? md.utils.escapeHtml(str)
  } else {
    highlighted = md.utils.escapeHtml(str)
  }

  const canRun = lang === 'html' || lang === 'svg'
  const isMermaid = lang === 'mermaid'
  const rawB64 = btoa(unescape(encodeURIComponent(str)))
  const extraClass = isMermaid ? ' mermaid-block' : ''

  const copyIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>`
  const runIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" x2="21" y1="14" y2="3"/></svg>`
  const codeIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>`

  return `<div class="code-block${extraClass}" data-raw="${rawB64}" data-lang="${langLabel}">
<div class="code-header"><span class="code-lang">${langLabel}</span><div class="code-actions">
<button class="code-btn copy-btn" data-action="copy" title="Copy code">${copyIcon}</button>${canRun ? `<button class="code-btn run-btn" data-action="run" title="Open in new tab">${runIcon}</button>` : ''}${isMermaid ? `<button class="code-btn run-btn" data-action="run-mermaid" title="Toggle diagram">${codeIcon}</button>` : ''}
</div></div>
<pre class="hljs"><code>${highlighted}</code></pre></div>`
}

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;')
}

// Protect LaTeX blocks from markdown preprocessing by replacing them with HTML comment placeholders.
// Using HTML comments ensures markdown-it NEVER touches the content.
// Returns [protectedText, latexBlocks] to avoid race conditions with global state.
// Code blocks (```) are NOT protected - we skip them during LaTeX scanning instead.
function protectLatexBlocks(text: string): [string, Array<{ placeholder: string; latex: string }>] {
  const latexBlocks: Array<{ placeholder: string; latex: string }> = []
  let result = ''
  let counter = 0

  function makeLatexPlaceholder(latex: string): string {
    const key = `LATEX_PH_${counter++}_END`
    latexBlocks.push({ placeholder: `<!--${key}-->`, latex })
    return `<!--${key}-->`
  }

  let i = 0
  while (i < text.length) {
    // Check for code block start ``` - SKIP the entire code block
    if (text.substring(i, i + 3) === '```') {
      // Find the closing ```
      const endIdx = text.indexOf('```', i + 3)
      if (endIdx !== -1) {
        // Copy the entire code block as-is, don't scan inside for LaTeX
        result += text.substring(i, endIdx + 3)
        i = endIdx + 3
        continue
      }
    }

    // Check for display math $$...$$
    if (text.substring(i, i + 2) === '$$') {
      const endIdx = text.indexOf('$$', i + 2)
      if (endIdx !== -1 && endIdx !== i + 2) {
        const latexContent = text.substring(i, endIdx + 2)
        result += makeLatexPlaceholder(latexContent)
        i = endIdx + 2
        continue
      }
    }

    // Check for display math \[...\]
    if (text.substring(i, i + 2) === '\\[') {
      const endIdx = text.indexOf('\\]', i + 2)
      if (endIdx !== -1) {
        const latexContent = text.substring(i, endIdx + 2)
        result += makeLatexPlaceholder(latexContent)
        i = endIdx + 2
        continue
      }
    }

    // Check for inline math \( ... \)
    if (text.substring(i, i + 2) === '\\(') {
      const endIdx = text.indexOf('\\)', i + 2)
      if (endIdx !== -1) {
        const latexContent = text.substring(i, endIdx + 2)
        result += makeLatexPlaceholder(latexContent)
        i = endIdx + 2
        continue
      }
    }

    // Check for inline math $...$ (but not $$)
    if (text[i] === '$' && text[i + 1] !== '$') {
      // Look for closing $ on the same line
      let j = i + 1
      while (j < text.length && text[j] !== '$' && text[j] !== '\n') {
        j++
      }
      if (j < text.length && text[j] === '$' && text[j + 1] !== '$') {
        const latexContent = text.substring(i, j + 1)
        result += makeLatexPlaceholder(latexContent)
        i = j + 1
        continue
      }
    }

    result += text[i]
    i++
  }

  return [result, latexBlocks]
}

// Restore LaTeX blocks from placeholders and render them with KaTeX.
// If KaTeX hasn't loaded yet, triggers async load and returns HTML with
// invisible placeholders (comment tags). The caller must re-render when
// KaTeX becomes available via onLazyLoad callback.
function restoreLatexBlocks(html: string, blocks: Array<{ placeholder: string; latex: string }>): string {
  if (blocks.length === 0) return html

  // If KaTeX not yet loaded, kick off load and leave placeholders as raw LaTeX
  if (!katexModule) {
    prefetchKatex()
    // Return HTML with placeholders replaced by styled error spans so content
    // is at least visible (raw LaTeX source)
    for (const { placeholder, latex } of blocks) {
      const escapedPlaceholder = escapeHtml(placeholder)
      const rawLatex = escapeHtml(latex)
      if (html.includes(escapedPlaceholder)) {
        html = html.replace(escapedPlaceholder, `<span class="katex-error">${rawLatex}</span>`)
      } else {
        html = html.replace(placeholder, `<span class="katex-error">${rawLatex}</span>`)
      }
    }
    return html
  }

  for (const { placeholder, latex } of blocks) {
    // Strip markdown delimiters ($$ or \[ or $ or \() for KaTeX input
    let math = latex
    const isDisplay = latex.startsWith('$$') || latex.startsWith('\\[')
    if ((latex.startsWith('$$') && latex.endsWith('$$')) ||
        (latex.startsWith('\\[') && latex.endsWith('\\]'))) {
      math = latex.slice(2, -2)
    } else if (latex.startsWith('$') && latex.endsWith('$')) {
      math = latex.slice(1, -1)
    } else if (latex.startsWith('\\(') && latex.endsWith('\\)')) {
      math = latex.slice(2, -2)
    }

    const displayMode = isDisplay
    try {
      const rendered = katexModule.renderToString(math.trim(), {
        displayMode,
        throwOnError: false,
        output: 'html',
      })
      // Try to replace the placeholder, but also handle HTML-escaped version
      // because markdown-it with html:false will escape < and > to &lt; &gt;
      const escapedPlaceholder = escapeHtml(placeholder)
      if (html.includes(escapedPlaceholder)) {
        html = html.replace(escapedPlaceholder, rendered)
      } else {
        html = html.replace(placeholder, rendered)
      }
    } catch (e) {
      console.error('KaTeX error:', e)
      const escapedPlaceholder = escapeHtml(placeholder)
      if (html.includes(escapedPlaceholder)) {
        html = html.replace(escapedPlaceholder, `<span class="katex-error">${escapeHtml(latex)}</span>`)
      } else {
        html = html.replace(placeholder, `<span class="katex-error">${escapeHtml(latex)}</span>`)
      }
    }
  }
  return html
}

/** Render markdown content to HTML (uses shared singleton) */
export function renderMarkdown(content: string): string {
  // Step 1: Protect LaTeX blocks, but skip code blocks (```)
  // We don't protect code blocks, we just skip them during LaTeX scanning
  const [latexProtected, latexBlocks] = protectLatexBlocks(content)
  // Step 2: Run markdown preprocessing (e.g. bold fix) — it won't touch LaTeX now
  const preprocessed = preprocessBoldMarkdown(latexProtected)
  // Step 3: Markdown render
  let html = md.render(preprocessed)
  // Step 4: Restore & render LaTeX blocks
  html = restoreLatexBlocks(html, latexBlocks)
  return html
}

export {hljs}
