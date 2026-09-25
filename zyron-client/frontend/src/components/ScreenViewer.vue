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
        <!-- Botão Alternar Ponteiro Laser -->
        <button
          @click="toggleLaser"
          type="button"
          :disabled="!hasVideo"
          :class="[
            !hasVideo
              ? 'opacity-40 cursor-not-allowed bg-slate-800 text-slate-500 border-slate-700/50'
              : isLaserActive
                ? 'bg-rose-500/25 text-rose-300 border-rose-500/50 shadow-sm shadow-rose-500/30 ring-1 ring-rose-500/60 cursor-pointer'
                : 'bg-slate-800 text-slate-300 border-slate-700 hover:text-white hover:bg-slate-700 cursor-pointer'
          ]"
          class="px-2.5 py-1.5 rounded-lg border text-xs font-semibold transition flex items-center space-x-1.5"
          :title="!hasVideo ? 'Aguardando início do compartilhamento de tela...' : (isLaserActive ? 'Desativar Ponteiro Laser (Atalho: L)' : 'Ativar Ponteiro Laser na tela do cliente (Atalho: L)')"
        >
          <span
            class="w-2 h-2 rounded-full"
            :class="!hasVideo ? 'bg-slate-600' : isLaserActive ? 'bg-rose-400 animate-ping' : 'bg-slate-500'"
          ></span>
          <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5 text-rose-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <circle cx="12" cy="12" r="3" stroke-width="2" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v3m0 12v3m9-9h-3M6 12H3" />
          </svg>
          <span>{{ !hasVideo ? 'Laser Indisponível' : isLaserActive ? 'Laser Ativo' : 'Ativar Laser' }}</span>
        </button>

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
    <div
      class="flex-1 bg-black relative flex items-center justify-center overflow-hidden"
      ref="videoContainerRef"
      @mousemove="onMouseMove"
      @mousedown="onMouseDown"
      @mouseleave="onMouseLeave"
      :class="{ 'cursor-crosshair': isLaserActive }"
    >
      <!-- Player de Vídeo -->
      <video
        ref="videoRef"
        id="clientScreenViewer"
        autoplay
        playsinline
        @loadedmetadata="hasVideo = true"
        class="w-full h-full object-contain pointer-events-none"
        :class="{ 'opacity-0': !hasVideo, 'opacity-100': hasVideo }"
      ></video>

      <!-- Laser Visual Local (Renderizado na tela do Operador) -->
      <div
        v-if="isLaserActive && localLaser.visible"
        class="pointer-events-none absolute z-30 transition-transform duration-75 ease-out"
        :style="{
          left: `${localLaser.x}px`,
          top: `${localLaser.y}px`,
          transform: 'translate(-50%, -50%)'
        }"
      >
        <!-- Pulso de Clique -->
        <div
          v-if="localLaser.isClicking"
          class="absolute -inset-4 rounded-full border-2 border-amber-300 animate-ping"
        ></div>
        <!-- Halo Externo -->
        <div class="w-8 h-8 rounded-full border-2 border-rose-500 bg-rose-500/25 shadow-lg shadow-rose-500/60 flex items-center justify-center animate-pulse">
          <!-- Ponto Central -->
          <div class="w-2.5 h-2.5 rounded-full bg-rose-300 shadow-md shadow-white"></div>
        </div>
        <!-- Tag do Operador -->
        <div class="absolute left-6 -top-2 whitespace-nowrap bg-rose-950/90 border border-rose-500/50 text-rose-200 text-[10px] font-mono px-1.5 py-0.5 rounded shadow-lg pointer-events-none">
          Laser Ativo
        </div>
      </div>

      <!-- Áudio Remoto do Cliente (Oculto) -->
      <audio ref="audioRef" id="clientVoiceAudio" autoplay playsinline></audio>

      <!-- Estado: Aguardando fluxo de vídeo -->
      <div
        v-if="!hasVideo"
        class="absolute inset-0 flex flex-col items-center justify-center p-6 text-center bg-slate-950 pointer-events-none"
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

const emit = defineEmits(['closeSession', 'laserUpdate'])

const videoRef = ref(null)
const audioRef = ref(null)
const containerRef = ref(null)
const videoContainerRef = ref(null)
const isFullscreen = ref(false)
const hasVideo = ref(false)

// Estado do Ponteiro Laser do Operador
const isLaserActive = ref(false) // Inicia desativado por padrão
const localLaser = ref({ visible: false, x: 0, y: 0, isClicking: false })
let lastSendTime = 0
let clickTimeout = null

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

function toggleLaser() {
  // Permite ativar o laser somente após o compartilhamento de tela estar ativo
  if (!hasVideo.value) return

  isLaserActive.value = !isLaserActive.value
  if (!isLaserActive.value) {
    localLaser.value.visible = false
    emit('laserUpdate', { x: 0, y: 0, active: false, click: false })
  }
}

/**
 * Calcula as coordenadas normalizadas (0.0 a 1.0) considerando o letterbox (object-contain) do vídeo
 */
function calculateNormalizedCoords(event) {
  if (!videoRef.value || !videoContainerRef.value) return null
  const video = videoRef.value
  const container = videoContainerRef.value

  const rect = container.getBoundingClientRect()
  const containerW = rect.width
  const containerH = rect.height

  // Dimensões do vídeo real recebido da tela do cliente
  const videoW = video.videoWidth || containerW
  const videoH = video.videoHeight || containerH

  if (videoW <= 0 || videoH <= 0 || containerW <= 0 || containerH <= 0) {
    return null
  }

  const videoRatio = videoW / videoH
  const containerRatio = containerW / containerH

  let renderW = containerW
  let renderH = containerH
  let offsetX = 0
  let offsetY = 0

  if (containerRatio > videoRatio) {
    // Barras pretas laterais (letterbox vertical)
    renderH = containerH
    renderW = containerH * videoRatio
    offsetX = (containerW - renderW) / 2
  } else {
    // Barras pretas superior/inferior (pillarbox horizontal)
    renderW = containerW
    renderH = containerW / videoRatio
    offsetY = (containerH - renderH) / 2
  }

  const mouseX = event.clientX - rect.left - offsetX
  const mouseY = event.clientY - rect.top - offsetY

  // Se o mouse estiver nas barras pretas fora do frame real de vídeo
  if (mouseX < 0 || mouseX > renderW || mouseY < 0 || mouseY > renderH) {
    return null
  }

  return {
    normX: Math.max(0, Math.min(1, mouseX / renderW)),
    normY: Math.max(0, Math.min(1, mouseY / renderH)),
    localX: event.clientX - rect.left,
    localY: event.clientY - rect.top
  }
}

function onMouseMove(event) {
  if (!isLaserActive.value || !hasVideo.value) return
  const coords = calculateNormalizedCoords(event)

  if (!coords) {
    localLaser.value.visible = false
    emit('laserUpdate', { x: 0, y: 0, active: false, click: false })
    return
  }

  // Atualiza posição do laser no visor do operador
  localLaser.value.visible = true
  localLaser.value.x = coords.localX
  localLaser.value.y = coords.localY

  // Throttle de ~30 fps para envio suave sem sobrecarga de rede
  const now = performance.now()
  if (now - lastSendTime >= 30) {
    lastSendTime = now
    emit('laserUpdate', {
      x: coords.normX,
      y: coords.normY,
      active: true,
      click: false
    })
  }
}

function onMouseDown(event) {
  if (!isLaserActive.value || !hasVideo.value) return
  const coords = calculateNormalizedCoords(event)
  if (!coords) return

  // Dispara animação de onda expansiva no operador
  localLaser.value.isClicking = true
  if (clickTimeout) clearTimeout(clickTimeout)
  clickTimeout = setTimeout(() => {
    localLaser.value.isClicking = false
  }, 350)

  // Envia pulso de clique imediatamente para a tela do cliente
  emit('laserUpdate', {
    x: coords.normX,
    y: coords.normY,
    active: true,
    click: true
  })
}

function onMouseLeave() {
  if (!isLaserActive.value) return
  localLaser.value.visible = false
  emit('laserUpdate', { x: 0, y: 0, active: false, click: false })
}

function onKeyDown(e) {
  // Atalho 'L' para alternar o laser sem tirar o foco da chamada (somente após compartilhamento ativo)
  if ((e.key === 'l' || e.key === 'L') && !['INPUT', 'TEXTAREA'].includes(e.target.tagName)) {
    if (hasVideo.value) {
      toggleLaser()
    }
  }
}

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
      if (isLaserActive.value) {
        isLaserActive.value = false
        localLaser.value.visible = false
        emit('laserUpdate', { x: 0, y: 0, active: false, click: false })
      }
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

  window.addEventListener('keydown', onKeyDown)
})
</script>
