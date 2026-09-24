<template>
  <div class="h-full flex flex-col items-center justify-center p-8 max-w-xl mx-auto">
    <!-- Estado: Ocioso (Pronto para solicitar) -->
    <div
      v-if="ticketStatus === 'idle'"
      class="w-full bg-slate-800/60 border border-slate-700/60 rounded-3xl p-8 text-center backdrop-blur shadow-xl relative overflow-hidden"
    >
      <div class="w-20 h-20 rounded-2xl bg-indigo-500/10 border border-indigo-500/20 text-indigo-400 mx-auto flex items-center justify-center mb-6">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-10 h-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
        </svg>
      </div>

      <h2 class="text-2xl font-bold text-white mb-2">Central de Suporte</h2>
      <p class="text-sm text-slate-400 max-w-sm mx-auto mb-8">
        Precisa de suporte? Clique no botão abaixo para entrar na fila.
      </p>

      <button
        @click="$emit('requestSupport')"
        :disabled="isConnecting"
        class="w-full py-4 px-6 rounded-2xl bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white font-semibold text-base shadow-lg shadow-indigo-600/30 active:scale-[0.99] transition duration-200 flex items-center justify-center space-x-3 disabled:opacity-50"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 5.636l-3.536 3.536m0 5.656l3.536 3.536M9.172 9.172L5.636 5.636m3.536 9.192l-3.536 3.536M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-5 0a4 4 0 11-8 0 4 4 0 018 0z" />
        </svg>
        <span>Solicitar Suporte</span>
      </button>
    </div>

    <!-- Estado: Na Fila de Espera -->
    <div
      v-else-if="ticketStatus === 'waiting'"
      class="w-full bg-slate-800/70 border border-slate-700/80 rounded-3xl p-8 text-center backdrop-blur shadow-2xl relative overflow-hidden"
    >
      <div class="absolute -top-12 -right-12 w-40 h-40 bg-indigo-500/10 rounded-full blur-2xl pointer-events-none"></div>

      <!-- Indicador da Posição na Fila -->
      <div class="relative mb-6">
        <div class="w-28 h-28 rounded-full bg-indigo-500/20 border-2 border-indigo-500/40 text-indigo-400 mx-auto flex flex-col items-center justify-center shadow-inner">
          <span class="text-4xl font-extrabold text-white tracking-tight">{{ position }}º</span>
          <span class="text-[11px] font-semibold text-indigo-300 uppercase tracking-widest mt-0.5">na fila</span>
        </div>
        <!-- Pulse ring -->
        <div class="absolute inset-0 w-28 h-28 mx-auto rounded-full border border-indigo-400/40 animate-ping pointer-events-none opacity-40"></div>
      </div>

      <h3 class="text-xl font-bold text-white mb-2">
        {{ position === 1 ? 'Você é o próximo!' : `Aguardando atendimento...` }}
      </h3>

      <p class="text-xs text-slate-400 mb-6 max-w-sm mx-auto">
        Você pode minimizar esta janela. Ela reaparecerá automaticamente na tela assim que o atendimento for aceito.
      </p>

      <div class="flex items-center space-x-3">
        <button
          @click="$emit('minimizeWindow')"
          type="button"
          class="flex-1 py-3 px-4 rounded-xl bg-slate-700/80 hover:bg-slate-700 text-slate-200 border border-slate-600 text-sm font-medium transition duration-150 flex items-center justify-center space-x-2"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-slate-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 14l-7 7m0 0l-7-7m7 7V3" />
          </svg>
          <span>Minimizar Janela</span>
        </button>

        <button
          @click="$emit('cancelTicket')"
          type="button"
          class="flex-1 py-3 px-4 rounded-xl bg-rose-500/10 hover:bg-rose-500/20 text-rose-300 hover:text-rose-200 border border-rose-500/20 hover:border-rose-500/40 text-sm font-medium transition duration-150 flex items-center justify-center space-x-2"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
          <span>Cancelar</span>
        </button>
      </div>
    </div>

    <!-- Estado: Chamado em Atendimento Ativo -->
    <div
      v-else-if="ticketStatus === 'in_call'"
      class="w-full bg-slate-800/80 border border-emerald-500/30 rounded-3xl p-8 text-center backdrop-blur shadow-2xl relative"
    >
      <div class="w-16 h-16 rounded-2xl bg-emerald-500/20 border border-emerald-500/30 text-emerald-400 mx-auto flex items-center justify-center mb-4">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-8 h-8 animate-pulse" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
        </svg>
      </div>

      <div class="inline-flex items-center space-x-2 bg-emerald-500/20 text-emerald-300 px-3 py-1 rounded-full text-xs font-semibold uppercase tracking-wider mb-2">
        <span class="w-2 h-2 rounded-full bg-emerald-400 animate-ping"></span>
        <span>Sessão Ativa</span>
      </div>

      <h3 class="text-xl font-bold text-white mb-1">Conectado</h3>
      <p class="text-sm text-slate-200 mb-4 font-semibold">{{ operatorName || operatorUser }}</p>

      <!-- Alerta visual quando a tela inteira ainda está sendo solicitada/aguardando aceite -->
      <div
        v-if="isAwaitingScreen"
        class="bg-amber-500/15 border border-amber-500/30 rounded-2xl p-3.5 mb-4 text-xs text-amber-200 text-left flex items-start space-x-2.5 animate-pulse"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5 shrink-0 text-amber-400 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
        <div>
          <strong class="font-semibold text-amber-300 block mb-0.5">Autorização Obrigatória da Tela Inteira:</strong>
          <span>Por favor, selecione <strong>"Tela Inteira"</strong> na janela do Windows e clique em Compartilhar. Se cancelar, a solicitação reaparecerá até o aceite.</span>
        </div>
      </div>

      <div class="text-xs text-slate-400 bg-slate-900/60 border border-slate-700/60 rounded-xl p-4 mb-6 text-left">
        <div class="flex items-center justify-between">
          <span>Tempo de Atendimento:</span>
          <span class="font-mono text-emerald-400 font-semibold">{{ formattedDuration }}</span>
        </div>
      </div>

      <!-- Controles durante a sessão -->
      <div class="flex items-center space-x-3">
        <button
          @click="$emit('compactWindow')"
          type="button"
          title="Minimizar para janela flutuante estilo Google Meet"
          class="py-3 px-4 rounded-xl bg-slate-700/70 hover:bg-slate-700 text-slate-200 border border-slate-600 text-sm font-medium transition duration-150 flex items-center justify-center space-x-2"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
          </svg>
          <span>Modo Flutuante</span>
        </button>

        <button
          @click="$emit('toggleMute')"
          :class="isMuted ? 'bg-amber-500/20 text-amber-300 border-amber-500/40' : 'bg-slate-700/70 text-slate-200 border-slate-600'"
          class="flex-1 py-3 px-4 rounded-xl border text-sm font-medium transition duration-150 flex items-center justify-center space-x-2"
        >
          <svg v-if="!isMuted" xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z" />
          </svg>
          <svg v-else xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" clip-rule="evenodd" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2" />
          </svg>
          <span>{{ isMuted ? 'Mudo' : 'Mutar' }}</span>
        </button>

        <button
          @click="$emit('closeSession')"
          class="flex-1 py-3 px-4 rounded-xl bg-rose-600 hover:bg-rose-500 text-white font-medium text-sm transition duration-150 shadow-md shadow-rose-600/30 flex items-center justify-center space-x-2"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 8l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2M5 3a2 2 0 00-2 2v1c0 8.284 6.716 15 15 15h1a2 2 0 002-2v-3.28a1 1 0 00-.684-.948l-4.493-1.498a1 1 0 00-1.21.502l-1.13 2.257a11.042 11.042 0 01-5.516-5.517l2.257-1.128a1 1 0 00.502-1.21L9.228 3.683A1 1 0 008.279 3H5z" />
          </svg>
          <span>Encerrar</span>
        </button>
      </div>

      <!-- Áudio Remoto do Operador (Oculto sem controls) -->
      <audio ref="audioRef" id="clientWaitingAudio" autoplay playsinline></audio>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'

const props = defineProps({
  ticketStatus: {
    type: String,
    default: 'idle' // idle | waiting | in_call
  },
  position: {
    type: Number,
    default: 0
  },
  ticketId: {
    type: String,
    default: ''
  },
  operatorUser: {
    type: String,
    default: ''
  },
  operatorName: {
    type: String,
    default: ''
  },
  callDurationSeconds: {
    type: Number,
    default: 0
  },
  isMuted: {
    type: Boolean,
    default: false
  },
  isConnecting: {
    type: Boolean,
    default: false
  },
  isAwaitingScreen: {
    type: Boolean,
    default: false
  },
  audioStream: Object
})

defineEmits(['requestSupport', 'cancelTicket', 'closeSession', 'toggleMute', 'minimizeWindow', 'compactWindow'])

const audioRef = ref(null)

const formattedDuration = computed(() => {
  const mins = Math.floor(props.callDurationSeconds / 60)
  const secs = props.callDurationSeconds % 60
  return `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`
})

watch(
  () => props.audioStream,
  (stream) => {
    if (audioRef.value && stream) {
      audioRef.value.srcObject = stream
      audioRef.value.volume = 1.0
      audioRef.value.play().catch(console.warn)
    }
  },
  { immediate: true }
)

onMounted(() => {
  if (audioRef.value && props.audioStream) {
    audioRef.value.srcObject = props.audioStream
    audioRef.value.volume = 1.0
    audioRef.value.play().catch(console.warn)
  }
})
</script>
