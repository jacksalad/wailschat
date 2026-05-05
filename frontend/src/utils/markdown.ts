import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js/lib/core'
import katex from 'katex'
import 'mermaid'

// Register mermaid language (no syntax highlighting needed, just prevents errors)
hljs.registerLanguage('mermaid', () => ({
  keywords: '',
  contains: [],
}))

// Register only commonly used languages to keep bundle small (~50KB vs ~500KB for full import)
import javascript from 'highlight.js/lib/languages/javascript'
import typescript from 'highlight.js/lib/languages/typescript'
import python from 'highlight.js/lib/languages/python'
import go from 'highlight.js/lib/languages/go'
import java from 'highlight.js/lib/languages/java'
import c from 'highlight.js/lib/languages/c'
import cpp from 'highlight.js/lib/languages/cpp'
import csharp from 'highlight.js/lib/languages/csharp'
import rust from 'highlight.js/lib/languages/rust'
import ruby from 'highlight.js/lib/languages/ruby'
import php from 'highlight.js/lib/languages/php'
import sql from 'highlight.js/lib/languages/sql'
import bash from 'highlight.js/lib/languages/bash'
import json from 'highlight.js/lib/languages/json'
import xml from 'highlight.js/lib/languages/xml'
import yaml from 'highlight.js/lib/languages/yaml'
import markdown from 'highlight.js/lib/languages/markdown'
import css from 'highlight.js/lib/languages/css'
import diff from 'highlight.js/lib/languages/diff'
import shell from 'highlight.js/lib/languages/shell'
import matlab from 'highlight.js/lib/languages/matlab'
import lua from 'highlight.js/lib/languages/lua'
import r from 'highlight.js/lib/languages/r'
import scala from 'highlight.js/lib/languages/scala'
import kotlin from 'highlight.js/lib/languages/kotlin'
import swift from 'highlight.js/lib/languages/swift'
import perl from 'highlight.js/lib/languages/perl'
import delphi from 'highlight.js/lib/languages/delphi'
import objectivec from 'highlight.js/lib/languages/objectivec'
import makefile from 'highlight.js/lib/languages/makefile'
import nginx from 'highlight.js/lib/languages/nginx'
import dockerfile from 'highlight.js/lib/languages/dockerfile'
import vim from 'highlight.js/lib/languages/vim'
import latex from 'highlight.js/lib/languages/latex'
import ini from 'highlight.js/lib/languages/ini'

hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('js', javascript)
hljs.registerLanguage('typescript', typescript)
hljs.registerLanguage('ts', typescript)
hljs.registerLanguage('python', python)
hljs.registerLanguage('py', python)
hljs.registerLanguage('go', go)
hljs.registerLanguage('golang', go)
hljs.registerLanguage('java', java)
hljs.registerLanguage('c', c)
hljs.registerLanguage('cpp', cpp)
hljs.registerLanguage('c++', cpp)
hljs.registerLanguage('csharp', csharp)
hljs.registerLanguage('cs', csharp)
hljs.registerLanguage('rust', rust)
hljs.registerLanguage('ruby', ruby)
hljs.registerLanguage('rb', ruby)
hljs.registerLanguage('php', php)
hljs.registerLanguage('sql', sql)
hljs.registerLanguage('bash', bash)
hljs.registerLanguage('sh', shell)
hljs.registerLanguage('shell', shell)
hljs.registerLanguage('json', json)
hljs.registerLanguage('xml', xml)
hljs.registerLanguage('html', xml)
hljs.registerLanguage('svg', xml)
hljs.registerLanguage('yaml', yaml)
hljs.registerLanguage('yml', yaml)
hljs.registerLanguage('markdown', markdown)
hljs.registerLanguage('md', markdown)
hljs.registerLanguage('css', css)
hljs.registerLanguage('diff', diff)
hljs.registerLanguage('matlab', matlab)
hljs.registerLanguage('lua', lua)
hljs.registerLanguage('r', r)
hljs.registerLanguage('scala', scala)
hljs.registerLanguage('kotlin', kotlin)
hljs.registerLanguage('swift', swift)
hljs.registerLanguage('perl', perl)
hljs.registerLanguage('delphi', delphi)
hljs.registerLanguage('objectivec', objectivec)
hljs.registerLanguage('objective-c', objectivec)
hljs.registerLanguage('objective-c++', objectivec)
hljs.registerLanguage('objc', objectivec)
hljs.registerLanguage('makefile', makefile)
hljs.registerLanguage('nginx', nginx)
hljs.registerLanguage('dockerfile', dockerfile)
hljs.registerLanguage('vim', vim)
hljs.registerLanguage('latex', latex)
hljs.registerLanguage('tex', latex)
hljs.registerLanguage('ini', ini)
hljs.registerLanguage('toml', ini)

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
  if (lang && hljs.getLanguage(lang)) {
    try {
      highlighted = hljs.highlight(str, {language: lang, ignoreIllegals: true}).value
    } catch {
      highlighted = md.utils.escapeHtml(str)
    }
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
  const copiedIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>`
  
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

// Restore LaTeX blocks from placeholders and render them with KaTeX
function restoreLatexBlocks(html: string, blocks: Array<{ placeholder: string; latex: string }>): string {
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
      const rendered = katex.renderToString(math.trim(), {
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
