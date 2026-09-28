<template>
  <div
    class="h-screen w-screen bg-slate-900 text-slate-100 flex flex-col justify-between p-3.5 select-none overflow-hidden rounded-2xl shadow-2xl relative transition-all duration-300"
    :class="containerBorderClass"
  >
    <!-- Barra Superior de Arraste estilo Google Meet -->
    <div
      style="--wails-draggable: drag; cursor: move;"
      class="flex items-center justify-between pb-2 border-b border-slate-800"
    >
      <div class="flex items-center space-x-2 overflow-hidden">
        <!-- Sinalizador Visual de Novo Chamado ou Status da Fila -->
        <span
          class="w-2.5 h-2.5 rounded-full shrink-0"
          :class="indicatorDotClass"
        ></span>
        <span
          class="text-[11px] font-bold tracking-wider uppercase truncate"
          :class="indicatorTextClass"
        >
          {{ indicatorLabel }}
        </span>
      </div>

      <!-- Botões de Controle da Janela Flutuante -->
      <div class="flex items-center space-x-1 shrink-0" style="--wails-draggable: no-drag;">
        <!-- Minimizar para Barra de Tarefas -->
        <button
          @click="$emit('minimizeWindow')"
          title="Minimizar para barra de tarefas"
          type="button"
          class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition active:scale-90"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M20 12H4" />
          </svg>
        </button>

        <!-- Expandir/Restaurar Janela Completa -->
        <button
          @click="$emit('expandWindow')"
          title="Restaurar janela completa"
          type="button"
          class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition active:scale-90"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Corpo: Quantidade de Pessoas na Fila e Sinal Visual -->
    <div
      style="--wails-draggable: drag; cursor: move;"
      class="flex flex-col justify-center space-y-2 py-1 px-1 flex-1"
    >
      <div class="flex items-center space-x-3.5">
        <!-- Badge Circular com a Quantidade -->
        <div
          class="w-12 h-12 rounded-2xl flex items-center justify-center font-black text-2xl shrink-0 transition-all duration-300 relative shadow-inner"
          :class="counterBadgeClass"
        >
          <span :class="{ 'animate-bounce': hasNewRequest }">{{ queueCount }}</span>
          <span
            v-if="queueCount > 0"
            class="absolute inset-0 rounded-2xl border pointer-events-none opacity-40 animate-ping"
            :class="hasNewRequest ? 'border-amber-400' : 'border-indigo-400'"
          ></span>
        </div>

        <!-- Descritivo da Fila -->
        <div class="truncate flex-1">
          <div class="text-[10px] text-slate-400 font-semibold uppercase tracking-wider leading-none mb-1">
            Pessoas na Fila
          </div>
          <div class="text-sm font-bold truncate">
            <span v-if="hasNewRequest" class="text-amber-300 flex items-center space-x-1">
              <span>Novo cliente aguardando!</span>
            </span>
            <span v-else-if="queueCount > 0" class="text-slate-100">
              {{ queueCount === 1 ? '1 cliente aguardando' : `${queueCount} clientes aguardando` }}
            </span>
            <span v-else class="text-slate-400 text-xs font-medium">
              Nenhum chamado no momento
            </span>
          </div>
        </div>
      </div>

      <!-- Alerta Visual em Destaque quando um Cliente Solicita Suporte -->
      <div
        v-if="hasNewRequest"
        class="bg-amber-500/20 border border-amber-500/40 rounded-xl px-2.5 py-1 text-[11px] font-semibold text-amber-200 flex items-center space-x-1.5 animate-pulse shadow-sm"
      >
        <span class="w-2 h-2 rounded-full bg-amber-400 animate-ping shrink-0"></span>
        <span class="truncate">Novo suporte solicitado!</span>
      </div>
    </div>

    <!-- Barra Inferior de Ação: Aceitar Próxima Chamada -->
    <div class="pt-2 border-t border-slate-800 flex items-center space-x-2" style="--wails-draggable: no-drag;">
      <!-- Botão Principal: Aceitar Próxima Chamada -->
      <button
        @click="handleAcceptNext"
        :disabled="queueCount === 0 || isAccepting"
        type="button"
        class="flex-1 py-2 px-3 rounded-xl font-semibold text-xs transition flex items-center justify-center space-x-2 shadow-md active:scale-95 disabled:opacity-40 disabled:cursor-not-allowed"
        :class="queueCount > 0 ? (hasNewRequest ? 'bg-gradient-to-r from-amber-500 to-indigo-600 hover:from-amber-400 hover:to-indigo-500 text-white shadow-amber-500/25 animate-pulse' : 'bg-gradient-to-r from-emerald-600 to-teal-500 hover:from-emerald-500 hover:to-teal-400 text-white shadow-emerald-600/30') : 'bg-slate-800 text-slate-400 border border-slate-700'"
        :title="queueCount > 0 ? 'Aceitar e abrir atendimento em tela cheia' : 'Nenhum cliente aguardando na fila'"
      >
        <svg
          v-if="isAccepting"
          class="animate-spin h-3.5 w-3.5 text-white"
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
          class="w-3.5 h-3.5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
        </svg>
        <span>{{ isAccepting ? 'Conectando...' : 'Aceitar próxima chamada' }}</span>
      </button>

      <!-- Botão Rápido para Restaurar Janela Cheia -->
      <button
        @click="$emit('expandWindow')"
        title="Restaurar janela completa"
        type="button"
        class="p-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white border border-slate-700/80 transition active:scale-95 shrink-0"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
        </svg>
      </button>
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

const emit = defineEmits(['acceptTicket', 'expandWindow', 'minimizeWindow'])

const queueCount = computed(() => {
  return Array.isArray(props.queue) ? props.queue.length : 0
})

function handleAcceptNext() {
  if (props.queue.length > 0 && !props.isAccepting) {
    emit('acceptTicket', props.queue[0])
  }
}

const containerBorderClass = computed(() => {
  if (props.hasNewRequest) {
    return 'border-2 border-amber-400 shadow-xl shadow-amber-500/30 ring-2 ring-amber-400/40 animate-pulse'
  }
  if (queueCount.value > 0) {
    return 'border border-indigo-500/50 shadow-indigo-500/20 shadow-lg'
  }
  return 'border border-slate-700/60'
})

const indicatorDotClass = computed(() => {
  if (props.hasNewRequest) {
    return 'bg-amber-400 animate-ping'
  }
  if (queueCount.value > 0) {
    return 'bg-indigo-400 animate-pulse'
  }
  return 'bg-emerald-400'
})

const indicatorTextClass = computed(() => {
  if (props.hasNewRequest) {
    return 'text-amber-400'
  }
  if (queueCount.value > 0) {
    return 'text-indigo-400'
  }
  return 'text-emerald-400'
})

const indicatorLabel = computed(() => {
  if (props.hasNewRequest) {
    return 'Novo Chamado!'
  }
  if (queueCount.value > 0) {
    return 'Fila de Atendimento'
  }
  return 'Operador Online'
})

const counterBadgeClass = computed(() => {
  if (props.hasNewRequest) {
    return 'bg-amber-500/20 border-2 border-amber-400 text-amber-300'
  }
  if (queueCount.value > 0) {
    return 'bg-indigo-500/20 border border-indigo-500/40 text-indigo-300'
  }
  return 'bg-slate-800 border border-slate-700 text-slate-400'
})
</script>
