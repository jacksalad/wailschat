<script lang="ts" setup>
import {ref, computed} from 'vue'
import {Check, X} from 'lucide-vue-next'
import type {SelectionRequest} from '../stores/message'

const props = defineProps<{
  request: SelectionRequest
}>()

const emit = defineEmits<{
  confirm: [requestID: string, selectedValues: string[]]
  cancel: [requestID: string]
}>()

const selectedRadio = ref<string | null>(null)
const selectedCheckbox = ref<string[]>([])

// Initialize defaults
if (props.request.default_value) {
  if (props.request.type === 'radio') {
    if (typeof props.request.default_value === 'string') {
      selectedRadio.value = props.request.default_value
    }
  } else {
    if (Array.isArray(props.request.default_value)) {
      selectedCheckbox.value = [...props.request.default_value]
    }
  }
}

const hasSelection = computed(() => {
  if (props.request.type === 'radio') return selectedRadio.value !== null
  return selectedCheckbox.value.length > 0
})

function confirm() {
  if (props.request.type === 'radio') {
    const values = selectedRadio.value ? [selectedRadio.value] : []
    emit('confirm', props.request.request_id, values)
  } else {
    emit('confirm', props.request.request_id, selectedCheckbox.value)
  }
}

function cancel() {
  emit('cancel', props.request.request_id)
}

function toggleCheckbox(value: string) {
  const idx = selectedCheckbox.value.indexOf(value)
  if (idx >= 0) {
    selectedCheckbox.value.splice(idx, 1)
  } else {
    selectedCheckbox.value.push(value)
  }
}

function isSelected(value: string): boolean {
  if (props.request.type === 'radio') return selectedRadio.value === value
  return selectedCheckbox.value.includes(value)
}
</script>

<template>
  <div class="selection-panel rounded-2xl px-4 py-3 max-w-[85%]">
    <!-- Prompt text -->
    <div class="flex items-start gap-2 mb-3">
      <div class="w-1 h-full min-h-[18px] rounded-full bg-blue-500/60 dark:bg-blue-400/50 mt-0.5 flex-shrink-0"></div>
      <p class="text-sm text-slate-700 dark:text-slate-300 leading-relaxed">{{ request.prompt }}</p>
    </div>

    <!-- Options -->
    <div class="space-y-1 mb-3">
      <template v-if="request.type === 'radio'">
        <label
          v-for="opt in request.options"
          :key="opt.value"
          :class="[
            'selection-option flex items-center gap-2.5 cursor-pointer py-2 px-3 rounded-xl transition-all duration-150',
            isSelected(opt.value)
              ? 'bg-blue-500/15 dark:bg-blue-500/20 border border-blue-300/50 dark:border-blue-500/30'
              : 'hover:bg-slate-100/80 dark:hover:bg-slate-600/30 border border-transparent'
          ]"
        >
          <div :class="[
            'w-4 h-4 rounded-full border-2 flex-shrink-0 flex items-center justify-center transition-colors',
            isSelected(opt.value)
              ? 'border-blue-500 dark:border-blue-400'
              : 'border-slate-300 dark:border-slate-500'
          ]">
            <div v-if="isSelected(opt.value)" class="w-2 h-2 rounded-full bg-blue-500 dark:bg-blue-400"></div>
          </div>
          <input type="radio" :value="opt.value" v-model="selectedRadio" class="hidden" />
          <span :class="[
            'text-sm transition-colors',
            isSelected(opt.value)
              ? 'text-blue-700 dark:text-blue-300 font-medium'
              : 'text-slate-600 dark:text-slate-400'
          ]">{{ opt.label }}</span>
        </label>
      </template>
      <template v-else>
        <label
          v-for="opt in request.options"
          :key="opt.value"
          :class="[
            'selection-option flex items-center gap-2.5 cursor-pointer py-2 px-3 rounded-xl transition-all duration-150',
            isSelected(opt.value)
              ? 'bg-blue-500/15 dark:bg-blue-500/20 border border-blue-300/50 dark:border-blue-500/30'
              : 'hover:bg-slate-100/80 dark:hover:bg-slate-600/30 border border-transparent'
          ]"
        >
          <div :class="[
            'w-4 h-4 rounded-md border-2 flex-shrink-0 flex items-center justify-center transition-colors',
            isSelected(opt.value)
              ? 'border-blue-500 dark:border-blue-400 bg-blue-500 dark:bg-blue-400'
              : 'border-slate-300 dark:border-slate-500'
          ]">
            <svg v-if="isSelected(opt.value)" class="w-2.5 h-2.5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7"/></svg>
          </div>
          <input type="checkbox" :value="opt.value" :checked="isSelected(opt.value)" @change="toggleCheckbox(opt.value)" class="hidden" />
          <span :class="[
            'text-sm transition-colors',
            isSelected(opt.value)
              ? 'text-blue-700 dark:text-blue-300 font-medium'
              : 'text-slate-600 dark:text-slate-400'
          ]">{{ opt.label }}</span>
        </label>
      </template>
    </div>

    <!-- Action buttons -->
    <div class="flex justify-end gap-2 pt-2 border-t border-slate-200/60 dark:border-slate-600/40">
      <button
        @click="cancel"
        class="selection-cancel-btn flex items-center gap-1 px-3 py-1.5 text-xs rounded-lg text-slate-500 dark:text-slate-400 hover:bg-slate-200/60 dark:hover:bg-slate-600/40 transition-colors"
      >
        <X class="w-3.5 h-3.5" />
        Cancel
      </button>
      <button
        @click="confirm"
        :disabled="!hasSelection"
        class="selection-confirm-btn flex items-center gap-1 px-3 py-1.5 text-xs rounded-lg bg-blue-600 hover:bg-blue-700 text-white disabled:opacity-30 disabled:cursor-not-allowed transition-all"
      >
        <Check class="w-3.5 h-3.5" />
        Confirm
      </button>
    </div>
  </div>
</template>

<style scoped>
.selection-panel {
  background-color: var(--ai-bubble-bg-light, rgba(241, 245, 249, 0.5));
}
:root.dark .selection-panel {
  background-color: var(--ai-bubble-bg-dark, rgba(51, 65, 85, 0.5));
}
</style>
