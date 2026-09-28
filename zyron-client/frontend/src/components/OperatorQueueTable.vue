<template>
  <div class="h-full flex flex-col items-center justify-center p-8 max-w-xl mx-auto select-none">
    <div
      class="w-full bg-slate-800/60 border rounded-3xl p-8 text-center backdrop-blur shadow-xl relative overflow-hidden transition-all duration-300"
      :class="hasNewRequest ? 'border-amber-500/60 shadow-amber-500/10' : 'border-slate-700/60'"
    >
      <div class="absolute -top-12 -right-12 w-40 h-40 bg-indigo-500/10 rounded-full blur-2xl pointer-events-none"></div>

      <!-- Indicador da Quantidade de Pessoas na Fila -->
      <div class="relative mb-6">
        <div
          class="w-28 h-28 rounded-full mx-auto flex flex-col items-center justify-center shadow-inner transition-colors duration-300"
          :class="queueCount > 0 ? (hasNewRequest ? 'bg-amber-500/20 border-2 border-amber-400 text-amber-300' : 'bg-indigo-500/20 border-2 border-indigo-500/40 text-indigo-400') : 'bg-slate-700/30 border-2 border-slate-600/40 text-slate-400'"
        >
          <span
            class="text-4xl font-extrabold tracking-tight transition-transform duration-300"
            :class="hasNewRequest ? 'text-amber-200 animate-bounce' : 'text-white'"
          >
            {{ queueCount }}
          </span>
          <span
            class="text-[11px] font-semibold uppercase tracking-widest mt-0.5"
            :class="hasNewRequest ? 'text-amber-300' : (queueCount > 0 ? 'text-indigo-300' : 'text-slate-400')"
          >
            {{ queueCount === 1 ? 'na fila' : 'na fila' }}
          </span>
        </div>

        <!-- Anel pulsante de sinalização visual quando há pessoas na fila -->
        <div
          v-if="queueCount > 0"
          class="absolute inset-0 w-28 h-28 mx-auto rounded-full border pointer-events-none opacity-40 animate-ping"
          :class="hasNewRequest ? 'border-amber-400' : 'border-indigo-400/50'"
        ></div>
      </div>

      <h2 class="text-2xl font-bold text-white mb-2">Painel de Atendimento</h2>

      <!-- Sinal Visual de Novo Chamado Solicitado -->
      <div
        v-if="hasNewRequest"
        class="inline-flex items-center space-x-2 bg-amber-500/20 border border-amber-500/40 text-amber-200 text-xs font-semibold px-3 py-1.5 rounded-full mb-3 animate-pulse"
      >
        <span class="w-2 h-2 rounded-full bg-amber-400 animate-ping"></span>
        <span>Novo suporte solicitado!</span>
      </div>

      <p class="text-sm text-slate-400 max-w-sm mx-auto mb-8">
        <span v-if="queueCount > 0">
          {{ queueCount === 1 ? 'Existe 1 pessoa aguardando na fila. Clique no botão abaixo para atender.' : `Existem ${queueCount} pessoas aguardando na fila. Clique no botão abaixo para atender.` }}
        </span>
        <span v-else>
          Nenhum atendimento na fila no momento.
        </span>
      </p>

      <!-- Botões de Ação do Operador -->
      <div class="flex flex-col space-y-3">
        <!-- Botão Principal: Atender Próximo -->
        <button
          @click="handleAcceptNext"
          :disabled="queueCount === 0 || isAccepting"
          class="w-full py-4 px-6 rounded-2xl bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white font-semibold text-base shadow-lg shadow-indigo-600/30 active:scale-[0.99] transition duration-200 flex items-center justify-center space-x-3 disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:from-indigo-600 disabled:hover:to-indigo-500 disabled:shadow-none"
        >
          <svg
            v-if="isAccepting"
            class="animate-spin -ml-1 mr-2 h-5 w-5 text-white"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          <svg
            v-else
            xmlns="http://www.w3.org/2000/svg"
            class="w-5 h-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
          </svg>
          <span>{{ isAccepting ? 'Conectando...' : 'Atender próximo' }}</span>
        </button>

        <!-- Botão Secundário: Modo Flutuante Compacto -->
        <button
          @click="$emit('enterFloatingMode')"
          type="button"
          title="Minimizar para caixa flutuante sobreposta (estilo Google Meet)"
          class="w-full py-2.5 px-4 rounded-xl bg-slate-700/60 hover:bg-slate-700 text-slate-300 hover:text-white border border-slate-600/70 text-xs font-medium transition duration-150 flex items-center justify-center space-x-2 active:scale-95"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
          </svg>
          <span>Minimizar para Modo Flutuante</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  queue: {
    type: Array,
    default: () => []
  },
  isAccepting: {
    type: Boolean,
    default: false
  },
  hasNewRequest: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['acceptTicket', 'enterFloatingMode'])

const queueCount = computed(() => {
  return Array.isArray(props.queue) ? props.queue.length : 0
})

function handleAcceptNext() {
  if (props.queue.length > 0 && !props.isAccepting) {
    emit('acceptTicket', props.queue[0])
  }
}
</script>
