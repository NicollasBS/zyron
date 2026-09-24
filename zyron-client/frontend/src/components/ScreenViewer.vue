<template>
  <div class="h-full flex flex-col bg-slate-950 text-white relative overflow-hidden select-none" ref="containerRef">
    <!-- Barra Superior da Sessão -->
    <div class="bg-slate-900/90 backdrop-blur border-b border-slate-800 px-4 py-2.5 flex items-center justify-between z-10">
      <div class="flex items-center space-x-3">
        <span class="inline-flex items-center space-x-1.5 bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 text-[11px] px-2.5 py-0.5 rounded-full font-medium">
          <span class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
          <span>Sessão Remota LAN</span>
        </span>
        <div class="text-xs text-slate-300">
          Atendendo: <strong class="text-white">{{ clientName || clientUser }}</strong>
        </div>
        <span class="text-slate-600">|</span>
        <div class="text-xs text-slate-400 font-mono">
          Duração: <span class="text-emerald-400 font-bold">{{ formattedDuration }}</span>
        </div>
      </div>

      <!-- Controles de Janela e Ações -->
      <div class="flex items-center space-x-2">
        <button
          @click="toggleFullscreen"
          class="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition text-xs flex items-center space-x-1"
          title="Tela Cheia"
        >
          <svg v-if="!isFullscreen" xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-5h-4m4 0v4m0-4l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
          </svg>
          <svg v-else xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>

        <button
          @click="$emit('closeSession')"
          class="px-3 py-1.5 rounded-lg bg-rose-600 hover:bg-rose-500 text-white font-medium text-xs shadow-sm hover:shadow-rose-600/30 transition flex items-center space-x-1.5"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
          <span>Encerrar Chamado</span>
        </button>
      </div>
    </div>

    <!-- Área de Visualização do Vídeo (Tela Remota) -->
    <div class="flex-1 bg-black relative flex items-center justify-center overflow-hidden">
      <!-- Player de Vídeo -->
      <video
        ref="videoRef"
        id="clientScreenViewer"
        autoplay
        playsinline
        class="w-full h-full object-contain"
        :class="{ 'opacity-0': !hasVideo, 'opacity-100': hasVideo }"
      ></video>

      <!-- Áudio Remoto do Cliente (Oculto) -->
      <audio ref="audioRef" id="clientVoiceAudio" autoplay playsinline></audio>

      <!-- Estado: Aguardando fluxo de vídeo -->
      <div
        v-if="!hasVideo"
        class="absolute inset-0 flex flex-col items-center justify-center p-6 text-center bg-slate-950"
      >
        <div class="w-16 h-16 rounded-2xl bg-indigo-500/10 border border-indigo-500/20 text-indigo-400 flex items-center justify-center mb-4">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-8 h-8 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
        </div>
        <h4 class="text-base font-semibold text-slate-200 mb-1">Aguardando compartilhamento de tela</h4>
        <p class="text-xs text-slate-400 max-w-sm">
          O cliente <strong>{{ clientName || clientUser }}</strong> está autorizando a captura de tela no Windows...
        </p>
      </div>
    </div>

    <!-- Barra Inferior de Controles de Voz -->
    <div class="bg-slate-900/90 backdrop-blur border-t border-slate-800 px-4 py-2 flex items-center justify-between z-10 text-xs">
      <div class="flex items-center space-x-3">
        <slot name="voiceControls"></slot>
      </div>

      <div class="text-[11px] text-slate-400 flex items-center space-x-2">
        <span>Resolução adaptativa LAN</span>
        <span>•</span>
        <span class="text-emerald-400">P2P Direto Ativo</span>
        <template v-if="displayVersion">
          <span>•</span>
          <span class="font-mono text-slate-400">{{ displayVersion }}</span>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'

const props = defineProps({
  clientUser: String,
  clientName: String,
  clientIp: String,
  durationSeconds: {
    type: Number,
    default: 0
  },
  videoStream: Object,
  audioStream: Object,
  appVersion: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['closeSession'])

const videoRef = ref(null)
const audioRef = ref(null)
const containerRef = ref(null)
const isFullscreen = ref(false)
const hasVideo = ref(false)

const formattedDuration = computed(() => {
  const mins = Math.floor(props.durationSeconds / 60)
  const secs = props.durationSeconds % 60
  return `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`
})

const displayVersion = computed(() => {
  if (!props.appVersion) return ''
  const v = props.appVersion
  return v.startsWith('v') || v.startsWith('V') ? v : `v${v}`
})

watch(
  () => props.videoStream,
  (stream) => {
    if (videoRef.value && stream) {
      videoRef.value.srcObject = stream
      hasVideo.value = true
      videoRef.value.play().catch((err) => {
        console.warn('Erro ao reproduzir stream de vídeo:', err)
      })
    } else {
      hasVideo.value = false
    }
  },
  { immediate: true }
)

watch(
  () => props.audioStream,
  (stream) => {
    if (audioRef.value && stream) {
      audioRef.value.srcObject = stream
      audioRef.value.play().catch((err) => {
        console.warn('Erro ao reproduzir áudio remoto:', err)
      })
    }
  },
  { immediate: true }
)

function toggleFullscreen() {
  if (!document.fullscreenElement) {
    if (containerRef.value?.requestFullscreen) {
      containerRef.value.requestFullscreen()
      isFullscreen.value = true
    }
  } else {
    if (document.exitFullscreen) {
      document.exitFullscreen()
      isFullscreen.value = false
    }
  }
}

onMounted(() => {
  if (videoRef.value && props.videoStream) {
    videoRef.value.srcObject = props.videoStream
    hasVideo.value = true
    videoRef.value.play().catch((err) => {
      console.warn('Erro ao reproduzir stream de vídeo no mount:', err)
    })
  }

  if (audioRef.value && props.audioStream) {
    audioRef.value.srcObject = props.audioStream
    audioRef.value.play().catch((err) => {
      console.warn('Erro ao reproduzir áudio remoto no mount:', err)
    })
  }

  document.addEventListener('fullscreenchange', () => {
    isFullscreen.value = !!document.fullscreenElement
  })
})
</script>
