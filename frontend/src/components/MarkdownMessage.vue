<script lang="ts" setup>
import {computed, ref, watch, onBeforeUnmount} from 'vue'
import {renderMarkdown, onLazyLoad, mermaidCollapsed, preprocessMermaidCodeForToggle} from '../utils/markdown'
import {BrowserOpenURL} from '../../wailsjs/runtime/runtime'

// Mermaid is rendered entirely inside renderMarkdown() via a reactive SVG cache
// (mermaidSvgMap). This component no longer imperatively patches the DOM after
// v-html binds — that old approach raced against v-html rebuilds (triggered by
// lazy hljs/KaTeX loads) and silently wrote SVGs into detached nodes, so
// diagrams intermittently failed to appear.

const props = defineProps<{
  content: string
  streaming?: boolean
  searchQuery?: string
}>()

const copiedIndex = ref(-1)
const containerRef = ref<HTMLElement | null>(null)

// Incrementing key to force re-render when lazy deps (KaTeX, hljs languages)
// finish loading, so their newly-available output gets rendered.
const renderRevision = ref(0)

const rendered = computed(() => {
  // Access renderRevision to create reactive dependency
  renderRevision.value
  return renderMarkdown(props.content)
})

// Register callback for lazy dependency loads (KaTeX, hljs languages).
// Mermaid does NOT need this — its SVG lands in the reactive map that
// `rendered` already reads, so it re-renders without this callback.
const unregisterLazyLoad = onLazyLoad(() => {
  renderRevision.value++
})

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
  unregisterLazyLoad()
  // Remove drag listeners that were attached to the persistent document.
  document.removeEventListener('mousemove', onDocMouseMove)
  document.removeEventListener('mouseup', onDocMouseUp)
})

// Copy button icons (toggled imperatively on click; buttons themselves come from v-html)
const copyIconSvg = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>`
const copiedIconSvg = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>`

// ── Mermaid zoom & drag (event delegation) ──
// Because v-html rebuilds the inner DOM on every render, we cannot attach
// listeners to individual .mermaid-interactive nodes. Instead the persistent
// container handles wheel/mousedown via delegation; transform state lives in a
// per-SVG WeakMap keyed by the (current) SVG element.
interface DragState {
  scale: number
  translateX: number
  translateY: number
}

const svgStates = new WeakMap<SVGElement, DragState>()
let dragging: { svg: SVGElement; state: DragState; startX: number; startY: number; baseTX: number; baseTY: number; wrapper: HTMLElement } | null = null

function applyTransform(svg: SVGElement, s: DragState) {
  svg.style.transform = `translate(${s.translateX}px, ${s.translateY}px) scale(${s.scale})`
  svg.style.transformOrigin = '0 0'
}

function getState(svg: SVGElement): DragState {
  let s = svgStates.get(svg)
  if (!s) {
    s = {scale: 1, translateX: 0, translateY: 0}
    svgStates.set(svg, s)
  }
  return s
}

// Wheel zoom — delegated from the persistent container.
function onWheel(e: WheelEvent) {
  const wrapper = (e.target as HTMLElement).closest('.mermaid-interactive') as HTMLElement | null
  if (!wrapper) return
  const svg = wrapper.querySelector('svg') as SVGElement | null
  if (!svg) return
  e.preventDefault()
  e.stopPropagation()

  const s = getState(svg)
  const rect = wrapper.getBoundingClientRect()
  const mouseX = e.clientX - rect.left
  const mouseY = e.clientY - rect.top
  const delta = e.deltaY > 0 ? 0.9 : 1.1
  const newScale = Math.min(Math.max(s.scale * delta, 0.1), 10)
  const scaleChange = newScale / s.scale
  s.translateX = mouseX - (mouseX - s.translateX) * scaleChange
  s.translateY = mouseY - (mouseY - s.translateY) * scaleChange
  s.scale = newScale
  applyTransform(svg, s)
}

// Drag (pan) start — delegated from the persistent container.
function onMouseDown(e: MouseEvent) {
  if (e.button !== 0) return
  const wrapper = (e.target as HTMLElement).closest('.mermaid-interactive') as HTMLElement | null
  if (!wrapper) return
  const svg = wrapper.querySelector('svg') as SVGElement | null
  if (!svg) return
  e.preventDefault()
  const s = getState(svg)
  dragging = {svg, state: s, startX: e.clientX, startY: e.clientY, baseTX: s.translateX, baseTY: s.translateY, wrapper}
  wrapper.style.cursor = 'grabbing'
  svg.style.pointerEvents = 'auto'
}

function onDocMouseMove(e: MouseEvent) {
  if (!dragging) return
  dragging.state.translateX = dragging.baseTX + (e.clientX - dragging.startX)
  dragging.state.translateY = dragging.baseTY + (e.clientY - dragging.startY)
  applyTransform(dragging.svg, dragging.state)
}

function onDocMouseUp() {
  if (!dragging) return
  dragging.wrapper.style.cursor = 'grab'
  dragging = null
}

// Attach drag tracking to the document (persistent) once.
document.addEventListener('mousemove', onDocMouseMove)
document.addEventListener('mouseup', onDocMouseUp)

function handleAction(e: MouseEvent) {
  // Route external links (http/https) through the system default browser first.
  // Falls back to window.open (new webview window) when BrowserOpenURL is not
  // available (e.g. wails runtime not injected). Other protocols (mailto:,
  // internal #anchors) keep default behavior.
  const linkEl = (e.target as HTMLElement).closest('a[data-external-link]') as HTMLAnchorElement | null
  if (linkEl) {
    const href = linkEl.getAttribute('href') || ''
    if (/^https?:\/\//i.test(href)) {
      e.preventDefault()
      try {
        BrowserOpenURL(href)
      } catch {
        window.open(href, '_blank')
      }
      return
    }
  }

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
      target.innerHTML = copiedIconSvg
      target.classList.add('copied')
      setTimeout(() => {
        target.innerHTML = copyIconSvg
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

  if (action === 'download') {
    const lang = block.dataset.lang || ''
    const ext = lang === 'svg' ? 'svg' : 'html'
    const d = new Date()
    const pad = (n: number) => String(n).padStart(2, '0')
    const ts = `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`
    const filename = `export_${ts}.${ext}`
    const mime = ext === 'svg' ? 'image/svg+xml' : 'text/html'
    const blob = new Blob([raw], {type: mime})
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  if (action === 'toggle-mermaid') {
    // Toggle between rendered SVG and source view via the reactive collapsed set.
    // The key is the preprocessed code; decode here from the raw block.
    const preprocessed = preprocessMermaidCodeForToggle(raw.trim())
    if (mermaidCollapsed.has(preprocessed)) {
      mermaidCollapsed.delete(preprocessed)
    } else {
      mermaidCollapsed.add(preprocessed)
    }
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
  <div
    class="markdown-body"
    ref="containerRef"
    v-html="highlightedHtml"
    @click="handleAction"
    @wheel="onWheel"
    @mousedown="onMouseDown"
  ></div>
</template>
