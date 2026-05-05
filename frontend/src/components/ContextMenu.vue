<script lang="ts" setup>
import { ref, watch, onMounted, onBeforeUnmount, nextTick, type Component } from 'vue'

export interface MenuItem {
  label?: string
  icon?: Component
  action?: () => void
  shortcut?: string
  disabled?: boolean
  danger?: boolean
  divider?: boolean
}

const props = defineProps<{
  visible: boolean
  x: number
  y: number
  items: MenuItem[]
}>()

const emit = defineEmits<{
  close: []
}>()

const menuRef = ref<HTMLElement | null>(null)
const adjustedPos = ref({ x: 0, y: 0 })

watch(() => props.visible, async (val) => {
  if (val) {
    await nextTick()
    clampPosition()
  }
})

function clampPosition() {
  const menu = menuRef.value
  if (!menu) {
    adjustedPos.value = { x: props.x, y: props.y }
    return
  }
  const rect = menu.getBoundingClientRect()
  const vw = window.innerWidth
  const vh = window.innerHeight
  let x = props.x
  let y = props.y
  if (x + rect.width > vw - 8) x = vw - rect.width - 8
  if (y + rect.height > vh - 8) y = vh - rect.height - 8
  if (x < 8) x = 8
  if (y < 8) y = 8
  adjustedPos.value = { x, y }
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}

function onDocClick(e: MouseEvent) {
  if (!props.visible) return
  if (menuRef.value && !menuRef.value.contains(e.target as Node)) {
    emit('close')
  }
}

onMounted(() => {
  document.addEventListener('keydown', onKeydown)
  document.addEventListener('click', onDocClick, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
  document.removeEventListener('click', onDocClick, true)
})
</script>

<template>
  <Teleport to="body">
    <div v-if="visible" class="context-menu-overlay" @contextmenu.prevent="emit('close')">
      <div
        ref="menuRef"
        class="context-menu"
        :style="{ left: adjustedPos.x + 'px', top: adjustedPos.y + 'px' }"
      >
        <template v-for="(item, idx) in items" :key="idx">
          <div v-if="item.divider" class="context-menu-divider" />
          <button
            v-else
            :disabled="item.disabled"
            :class="['context-menu-item', { 'text-red-500 dark:text-red-400': item.danger }]"
            @click="item.action?.(); emit('close')"
          >
            <component v-if="item.icon" :is="item.icon" class="context-menu-icon" />
            <span class="context-menu-label">{{ item.label }}</span>
            <span v-if="item.shortcut" class="context-menu-shortcut">{{ item.shortcut }}</span>
          </button>
        </template>
      </div>
    </div>
  </Teleport>
</template>

<style>
.context-menu-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
}

.context-menu {
  position: fixed;
  min-width: 180px;
  max-width: 280px;
  background: rgba(255, 255, 255, 0.92);
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  padding: 4px;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.12), 0 2px 8px rgba(0, 0, 0, 0.06);
  backdrop-filter: blur(12px);
  animation: context-menu-in 0.12s ease-out;
}

html.dark .context-menu {
  background: rgba(30, 41, 59, 0.92);
  border-color: #475569;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.4), 0 2px 8px rgba(0, 0, 0, 0.2);
}

.context-menu-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 6px 10px;
  border: none;
  background: none;
  border-radius: 6px;
  font-size: 13px;
  color: #334155;
  cursor: pointer;
  transition: background 0.1s;
  text-align: left;
  white-space: nowrap;
}

html.dark .context-menu-item {
  color: #cbd5e1;
}

.context-menu-item:hover:not(:disabled) {
  background: rgba(59, 130, 246, 0.08);
}

html.dark .context-menu-item:hover:not(:disabled) {
  background: rgba(59, 130, 246, 0.15);
}

.context-menu-item:disabled {
  opacity: 0.4;
  cursor: default;
}

.context-menu-icon {
  width: 15px;
  height: 15px;
  flex-shrink: 0;
  opacity: 0.7;
}

.context-menu-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.context-menu-shortcut {
  font-size: 11px;
  color: #94a3b8;
  margin-left: auto;
  padding-left: 16px;
}

html.dark .context-menu-shortcut {
  color: #64748b;
}

.context-menu-divider {
  height: 1px;
  background: #e2e8f0;
  margin: 4px 8px;
}

html.dark .context-menu-divider {
  background: #475569;
}

@keyframes context-menu-in {
  from {
    opacity: 0;
    transform: scale(0.95);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}
</style>
