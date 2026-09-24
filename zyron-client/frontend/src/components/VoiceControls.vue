<template>
  <div class="flex items-center space-x-4">
    <!-- Botão de Microfone (Mudo / Ativo) -->
    <button
      @click="$emit('toggleMute')"
      :class="isMuted ? 'bg-rose-500/20 text-rose-300 border-rose-500/40 hover:bg-rose-500/30' : 'bg-slate-800 hover:bg-slate-700 text-slate-200 border-slate-700'"
      class="px-3 py-1.5 rounded-lg border text-xs font-medium transition flex items-center space-x-1.5"
      :title="isMuted ? 'Ativar microfone' : 'Silenciar microfone'"
    >
      <svg v-if="!isMuted" xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
      </svg>
      <svg v-else xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-rose-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" clip-rule="evenodd" />
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2" />
      </svg>
      <span>{{ isMuted ? 'Microfone Silenciado' : 'Microfone Ativo' }}</span>
    </button>

    <!-- Controle de Volume do Alto-falante -->
    <div class="flex items-center space-x-2 text-slate-300">
      <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
      </svg>
      <input
        type="range"
        min="0"
        max="1"
        step="0.05"
        v-model="internalVolume"
        @input="onVolumeChange"
        class="w-24 h-1.5 bg-slate-700 rounded-lg appearance-none cursor-pointer accent-indigo-500"
        title="Volume de voz do cliente"
      />
      <span class="text-[11px] font-mono text-slate-400 w-8">{{ Math.round(internalVolume * 100) }}%</span>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const props = defineProps({
  isMuted: {
    type: Boolean,
    default: false
  },
  volume: {
    type: Number,
    default: 1.0
  }
})

const emit = defineEmits(['toggleMute', 'updateVolume'])

const internalVolume = ref(props.volume)

function onVolumeChange() {
  emit('updateVolume', parseFloat(internalVolume.value))
}
</script>
