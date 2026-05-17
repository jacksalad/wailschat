<script lang="ts" setup>
import {computed, ref, watch, onMounted, nextTick, onBeforeUnmount} from 'vue'
import {renderMarkdown} from '../utils/markdown'
import mermaid from 'mermaid'

// Initialize mermaid - theme will be dynamically set based on dark/light mode
mermaid.initialize({
  startOnLoad: false,
  theme: 'default',
  securityLevel: 'loose',
  fontFamily: 'inherit',
})

// Module-level SVG cache: keyed by preprocessed mermaid code → rendered SVG string
const mermaidSvgCache = new Map<string, string>()

const props = defineProps<{
  content: string
  streaming?: boolean
  searchQuery?: string
}>()

const copiedIndex = ref(-1)
const containerRef = ref<HTMLElement | null>(null)
let mermaidIdCounter = 0

// Track rendered mermaid blocks
const renderedMermaidBlocks = ref<Set<HTMLElement>>(new Set())

const rendered = computed(() => renderMarkdown(props.content))

// Debounced rendering for streaming: coalesce multiple chunks into single renders
const debouncedRendered = ref('')
let renderTimer: ReturnType<typeof setTimeout> | null = null

watch(rendered, (newHtml) => {
  if (props.streaming) {
    if (renderTimer) clearTimeout(renderTimer)
    renderTimer = setTimeout(() => {
      debouncedRendered.value = newHtml
    }, 150)
  } else {
    debouncedRendered.value = newHtml
  }
}, { immediate: true })

onBeforeUnmount(() => {
  if (renderTimer) clearTimeout(renderTimer)
})

// Check if text contains non-ASCII characters (Chinese, etc.)
function hasNonAscii(text: string): boolean {
  return /[^\x00-\x7F]/.test(text)
}

// Preprocess mermaid code to fix Chinese text issues
// Wraps text containing non-ASCII characters in [] or {} with double quotes
// Example: [中文] -> ["中文"], {中文?} -> {"中文?"}
function preprocessMermaidCode(code: string): string {
  // Process [] brackets: match [content] where content has non-ASCII and not already quoted
  let fixed = code.replace(/\[([^\]]*)\]/g, (match, content) => {
    // Skip if already quoted
    if (content.startsWith('"') || content.startsWith("'")) {
      return match
    }
    // Add quotes if contains non-ASCII
    if (hasNonAscii(content)) {
      return `["${content}"]`
    }
    return match
  })

  // Process {} braces: match {content} where content has non-ASCII and not already quoted
  fixed = fixed.replace(/\{([^}]*)\}/g, (match, content) => {
    // Skip if already quoted
    if (content.startsWith('"') || content.startsWith("'")) {
      return match
    }
    // Skip if it's a style/class directive like { ... }
    if (content.includes(':') || content.includes(';') || content.startsWith(' ')) {
      return match
    }
    // Add quotes if contains non-ASCII
    if (hasNonAscii(content)) {
      return `{"${content}"}`
    }
    return match
  })

  return fixed
}

// Shared SVG icons
const copyIconSvg = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>`
const codeIconSvg = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>`

// Apply a rendered SVG to a mermaid block (shared between cached and fresh paths)
function applyRenderedSvg(block: HTMLElement, svg: string) {
  const originalContent = block.querySelector('.code-header')?.nextElementSibling?.outerHTML || ''
  block.dataset.originalContent = originalContent

  // Create a container for the rendered diagram
  const wrapper = document.createElement('div')
  wrapper.className = 'mermaid-rendered'
  wrapper.innerHTML = svg

  // Add interactive container for zoom/pan
  const interactiveWrapper = document.createElement('div')
  interactiveWrapper.className = 'mermaid-interactive'
  interactiveWrapper.appendChild(wrapper)

  // Replace the original block content
  block.innerHTML = ''
  block.appendChild(interactiveWrapper)

  // Restore header with CODE button to switch back to code view
  const header = document.createElement('div')
  header.className = 'code-header'
  header.innerHTML = `
    <span class="code-lang">mermaid</span>
    <div class="code-actions">
      <button class="code-btn copy-btn" data-action="copy" title="Copy code">${copyIconSvg}</button>
      <button class="code-btn run-btn" data-action="toggle-mermaid" title="Show code">${codeIconSvg}</button>
    </div>
  `
  block.insertBefore(header, block.firstChild)
  block.classList.add('rendered')

  // Add wheel zoom and drag functionality
  setupMermaidInteraction(interactiveWrapper)

  renderedMermaidBlocks.value.add(block)
}

// Render a single mermaid block
async function renderMermaidBlock(block: HTMLElement, isAutoRender = false): Promise<boolean> {
  const rawB64 = block.dataset.raw || ''
  const code = decodeURIComponent(escape(atob(rawB64)))

  // Preprocess code to fix Chinese text
  const preprocessedCode = preprocessMermaidCode(code.trim())

  // Check cache first
  const cachedSvg = mermaidSvgCache.get(preprocessedCode)
  if (cachedSvg) {
    applyRenderedSvg(block, cachedSvg)
    return true
  }

  const id = `mermaid-${++mermaidIdCounter}-${Date.now()}`

  try {
    const { svg } = await mermaid.render(id, preprocessedCode)

    // Store in cache
    mermaidSvgCache.set(preprocessedCode, svg)

    // Store preprocessed code for retry
    block.dataset.preprocessedCode = preprocessedCode

    applyRenderedSvg(block, svg)
    return true
  } catch (e) {
    console.error('Mermaid render error:', e)
    // If auto-render failed, don't modify the block at all - keep original code with RUN button
    if (isAutoRender) {
      return false
    }

    // If manual toggle failed, show error but keep toggle button functional
    // Remove any partial rendering
    const savedContent = block.dataset.originalContent
    const fallbackContent = block.querySelector('pre')?.outerHTML || ''
    block.innerHTML = savedContent || fallbackContent || block.innerHTML
    block.classList.remove('rendered')
    block.classList.add('has-error')

    // Remove any existing error indicators first
    block.querySelectorAll('.mermaid-error').forEach(el => el.remove())

    // Show error indicator (auto-remove after 3 seconds)
    const errorDiv = document.createElement('div')
    errorDiv.className = 'mermaid-error'
    errorDiv.textContent = `Render error: ${e instanceof Error ? e.message : 'Unknown error'}`

    const header = block.querySelector('.code-header')
    if (header) {
      header.insertAdjacentElement('afterend', errorDiv)
    } else {
      block.appendChild(errorDiv)
    }

    setTimeout(() => errorDiv.remove(), 3000)

    // Keep RUN button working by restoring it
    const actionsDiv = block.querySelector('.code-actions')
    if (actionsDiv && !actionsDiv.querySelector('[data-action="run-mermaid"]')) {
      const runBtn = document.createElement('button')
      runBtn.className = 'code-btn run-btn'
      runBtn.dataset.action = 'run-mermaid'
      runBtn.title = 'Toggle diagram'
      runBtn.innerHTML = codeIconSvg
      actionsDiv.appendChild(runBtn)
    }

    return false
  }
}

// Toggle mermaid block between rendered and code view
async function toggleMermaidBlock(block: HTMLElement) {
  const wasRendered = block.classList.contains('rendered')

  if (wasRendered) {
    // Get code content from stored original or from DOM
    let codeHtml = block.dataset.originalContent
    if (!codeHtml) {
      // Fallback: get the pre element that contains the code
      const preEl = block.querySelector('pre')
      codeHtml = preEl?.outerHTML || ''
    }

    // Restore the code block with COPY and RUN buttons (no CODE button needed)
    block.innerHTML = ''
    const header = document.createElement('div')
    header.className = 'code-header'
    header.innerHTML = `
      <span class="code-lang">mermaid</span>
      <div class="code-actions">
        <button class="code-btn copy-btn" data-action="copy" title="Copy code">${copyIconSvg}</button>
        <button class="code-btn run-btn" data-action="toggle-mermaid" title="Toggle diagram">${codeIconSvg}</button>
      </div>
    `
    block.appendChild(header)
    block.insertAdjacentHTML('beforeend', codeHtml)
    block.classList.remove('rendered', 'has-error')
    renderedMermaidBlocks.value.delete(block)
  } else {
    // Switch to rendered view (manual toggle, not auto-render)
    block.classList.remove('has-error')
    await renderMermaidBlock(block, false)
  }
}

// Setup wheel zoom and drag functionality for mermaid container
function setupMermaidInteraction(container: HTMLElement) {
  let scale = 1
  let translateX = 0
  let translateY = 0
  let isDragging = false
  let startX = 0
  let startY = 0
  let lastTranslateX = 0
  let lastTranslateY = 0

  const svg = container.querySelector('svg')
  if (!svg) return

  // Enable pointer events on SVG
  svg.style.pointerEvents = 'auto'

  // Wheel zoom handler
  container.addEventListener('wheel', (e: WheelEvent) => {
    e.preventDefault()
    e.stopPropagation()

    const rect = container.getBoundingClientRect()
    const mouseX = e.clientX - rect.left
    const mouseY = e.clientY - rect.top

    // Calculate zoom
    const delta = e.deltaY > 0 ? 0.9 : 1.1
    const newScale = Math.min(Math.max(scale * delta, 0.1), 10)

    // Adjust translation to zoom towards mouse position
    const scaleChange = newScale / scale
    translateX = mouseX - (mouseX - translateX) * scaleChange
    translateY = mouseY - (mouseY - translateY) * scaleChange
    scale = newScale

    applyTransform()
  }, { passive: false })

  // Mouse drag handlers
  container.addEventListener('mousedown', (e: MouseEvent) => {
    if (e.button !== 0) return // Only left click
    e.preventDefault()
    isDragging = true
    startX = e.clientX
    startY = e.clientY
    lastTranslateX = translateX
    lastTranslateY = translateY
    container.style.cursor = 'grabbing'
  })

  document.addEventListener('mousemove', (e: MouseEvent) => {
    if (!isDragging) return
    const deltaX = e.clientX - startX
    const deltaY = e.clientY - startY
    translateX = lastTranslateX + deltaX
    translateY = lastTranslateY + deltaY
    applyTransform()
  })

  document.addEventListener('mouseup', () => {
    if (isDragging) {
      isDragging = false
      container.style.cursor = 'grab'
    }
  })

  // Apply CSS transform
  function applyTransform() {
    if (svg) {
      svg.style.transform = `translate(${translateX}px, ${translateY}px) scale(${scale})`
      svg.style.transformOrigin = '0 0'
    }
  }

  // Set initial cursor
  container.style.cursor = 'grab'
}

// Render all mermaid blocks sequentially to avoid blocking the main thread
async function renderMermaidBlocks() {
  if (!containerRef.value) return

  const blocks = Array.from(
    containerRef.value.querySelectorAll('.code-block.mermaid-block[data-lang="mermaid"]')
  ).filter(block => !renderedMermaidBlocks.value.has(block as HTMLElement)) as HTMLElement[]

  if (blocks.length === 0) return

  // Render one at a time, yielding to the browser between each
  for (const block of blocks) {
    await renderMermaidBlock(block, true)
    // Yield to browser to keep UI responsive
    await new Promise(resolve => setTimeout(resolve, 0))
  }
}

// Watch for rendered content changes — skip mermaid during streaming
watch(rendered, async () => {
  await nextTick()
  if (props.streaming) return
  renderedMermaidBlocks.value.clear()
  await renderMermaidBlocks()
})

// When streaming ends, render all mermaid blocks
watch(() => props.streaming, async (newVal, oldVal) => {
  if (oldVal === true && newVal === false) {
    await nextTick()
    renderedMermaidBlocks.value.clear()
    await renderMermaidBlocks()
  }
})

onMounted(async () => {
  await nextTick()
  if (!props.streaming) {
    await renderMermaidBlocks()
  }
})

function handleAction(e: MouseEvent) {
  const target = (e.target as HTMLElement).closest('[data-action]') as HTMLElement | null
  if (!target) return
  const block = target.closest('.code-block') as HTMLElement | null
  if (!block) return

  const action = target.dataset.action
  const rawB64 = block.dataset.raw || ''
  const raw = decodeURIComponent(escape(atob(rawB64)))

  if (action === 'copy') {
    navigator.clipboard.writeText(raw).then(() => {
      const idx = Array.from(block.parentElement?.querySelectorAll('.code-block') || []).indexOf(block)
      copiedIndex.value = idx
      const copyIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>`
      target.innerHTML = copyIcon
      target.classList.add('copied')
      setTimeout(() => {
        const origCopyIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>`
        target.innerHTML = origCopyIcon
        target.classList.remove('copied')
        if (copiedIndex.value === idx) copiedIndex.value = -1
      }, 1500)
    })
  }

  if (action === 'run') {
    const w = window.open('', '_blank')
    if (w) {
      w.document.open()
      if (block.dataset.lang === 'svg') {
        w.document.write(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>SVG Preview</title><style>body{margin:0;display:flex;justify-content:center;align-items:center;min-height:100vh;background:#1e293b;}</style></head><body>${raw}</body></html>`)
      } else {
        w.document.write(raw)
      }
      w.document.close()
    }
  }

  if (action === 'run-mermaid' || action === 'toggle-mermaid') {
    toggleMermaidBlock(block)
  }
}

// Search highlighting
function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

const highlightedHtml = computed(() => {
  const q = props.searchQuery?.trim()
  if (!q) return debouncedRendered.value
  const escaped = escapeRegex(q)
  const re = new RegExp(`(${escaped})`, 'gi')
  // Only replace text between > and < (visible text nodes in HTML)
  return debouncedRendered.value.replace(/>([^<]+)</g, (match, textContent: string) => {
    return '>' + textContent.replace(re, '<mark class="search-match">$1</mark>') + '<'
  })
})
</script>

<template>
  <div class="markdown-body" ref="containerRef" v-html="highlightedHtml" @click="handleAction"></div>
</template>
