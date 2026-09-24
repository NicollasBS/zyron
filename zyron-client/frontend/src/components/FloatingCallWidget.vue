<template>
  <div class="h-screen w-screen bg-slate-900 text-slate-100 flex flex-col justify-between p-3.5 select-none overflow-hidden border border-emerald-500/40 rounded-2xl shadow-2xl relative">
    <!-- Barra Superior de Arraste estilo Google Meet -->
    <div
      style="--wails-draggable: drag; cursor: move;"
      class="flex items-center justify-between pb-2 border-b border-slate-800"
    >
      <div class="flex items-center space-x-2">
        <span class="w-2.5 h-2.5 rounded-full bg-emerald-400 animate-ping"></span>
        <span class="text-[11px] font-bold text-emerald-400 tracking-wider uppercase">Chamada Ativa</span>
      </div>

      <!-- Botão para expandir/restaurar a janela cheia -->
      <button
        style="--wails-draggable: no-drag;"
        @click="$emit('expandWindow')"
        title="Restaurar janela completa"
        type="button"
        class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
        </svg>
      </button>
    </div>

    <!-- Corpo: Operador e Cronômetro -->
    <div
      style="--wails-draggable: drag; cursor: move;"
      class="flex items-center justify-between py-2 px-1"
    >
      <div class="flex items-center space-x-2.5 overflow-hidden">
        <div class="w-9 h-9 rounded-xl bg-emerald-500/20 border border-emerald-500/30 text-emerald-400 flex items-center justify-center shrink-0">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
          </svg>
        </div>
        <div class="truncate">
          <div class="text-[10px] text-slate-400 font-medium leading-none mb-1">Atendimento</div>
          <div class="text-xs font-bold text-white truncate max-w-[140px]">{{ operatorName || 'Suporte' }}</div>
        </div>
      </div>

      <div class="text-right shrink-0">
        <div class="text-[10px] text-slate-400 font-medium leading-none mb-1">Duração</div>
        <div class="font-mono text-xs font-bold text-emerald-400 bg-emerald-950/60 border border-emerald-500/20 px-2 py-0.5 rounded-md">
          {{ formattedDuration }}
        </div>
      </div>
    </div>

    <!-- Barra de Controles Inferiores -->
    <div class="flex items-center space-x-2 pt-2 border-t border-slate-800" style="--wails-draggable: no-drag;">
      <!-- Botão Mudo -->
      <button
        @click="$emit('toggleMute')"
        type="button"
        :class="isMuted ? 'bg-amber-500/20 text-amber-300 border-amber-500/40 hover:bg-amber-500/30' : 'bg-slate-800 text-slate-200 border-slate-700 hover:bg-slate-700'"
        class="flex-1 py-2 px-3 rounded-xl border text-xs font-semibold transition flex items-center justify-center space-x-1.5 active:scale-95"
      >
        <svg v-if="!isMuted" xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
        </svg>
        <svg v-else xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" clip-rule="evenodd" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2" />
        </svg>
        <span>{{ isMuted ? 'Mudo' : 'Mutar' }}</span>
      </button>

      <!-- Botão Encerrar Atendimento -->
      <button
        @click="$emit('closeSession')"
        type="button"
        class="flex-1 py-2 px-3 rounded-xl bg-rose-600 hover:bg-rose-500 text-white font-semibold text-xs transition flex items-center justify-center space-x-1.5 shadow-md shadow-rose-600/30 active:scale-95"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 8l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2M5 3a2 2 0 00-2 2v1c0 8.284 6.716 15 15 15h1a2 2 0 002-2v-3.28a1 1 0 00-.684-.948l-4.493-1.498a1 1 0 00-1.21.502l-1.13 2.257a11.042 11.042 0 01-5.516-5.517l2.257-1.128a1 1 0 00.502-1.21L9.228 3.683A1 1 0 008.279 3H5z" />
        </svg>
        <span>Encerrar</span>
      </button>
    </div>

    <!-- Áudio Remoto do Operador (Oculto sem controls) -->
    <audio ref="audioRef" id="floatingOperatorAudio" autoplay playsinline></audio>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'

const props = defineProps({
  operatorName: {
    type: String,
    default: ''
  },
  durationSeconds: {
    type: Number,
    default: 0
  },
  isMuted: {
    type: Boolean,
    default: false
  },
  audioStream: Object
})

defineEmits(['toggleMute', 'closeSession', 'expandWindow'])

const audioRef = ref(null)

const formattedDuration = computed(() => {
  const m = Math.floor(props.durationSeconds / 60)
  const s = props.durationSeconds % 60
  return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
})

watch(
  () => props.audioStream,
  (stream) => {
    if (audioRef.value && stream) {
      audioRef.value.srcObject = stream
      audioRef.value.volume = 1.0
      audioRef.value.play().catch((err) => {
        console.warn('[FloatingCallWidget] Erro ao reproduzir áudio do operador:', err)
      })
    }
  },
  { immediate: true }
)

onMounted(() => {
  if (audioRef.value && props.audioStream) {
    audioRef.value.srcObject = props.audioStream
    audioRef.value.volume = 1.0
    audioRef.value.play().catch((err) => {
      console.warn('[FloatingCallWidget] Erro ao reproduzir no onMounted:', err)
    })
  }
})
</script>
