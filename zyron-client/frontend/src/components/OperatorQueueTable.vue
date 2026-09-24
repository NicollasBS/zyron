<template>
  <div class="h-full flex flex-col items-center justify-center p-8 max-w-xl mx-auto select-none">
    <div
      class="w-full bg-slate-800/60 border border-slate-700/60 rounded-3xl p-8 text-center backdrop-blur shadow-xl relative overflow-hidden"
    >
      <!-- Ícone Central -->
      <div class="w-20 h-20 rounded-2xl bg-indigo-500/10 border border-indigo-500/20 text-indigo-400 mx-auto flex items-center justify-center mb-6">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-10 h-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
        </svg>
      </div>

      <h2 class="text-2xl font-bold text-white mb-2">Painel de Atendimento</h2>
      <p class="text-sm text-slate-400 max-w-sm mx-auto mb-8">
        <span v-if="queue.length > 0">
          Existe atendimento aguardando na fila. Clique no botão abaixo para atender.
        </span>
        <span v-else>
          Nenhum atendimento disponível no momento.
        </span>
      </p>

      <!-- Botão Único: Atender Próximo (Não clicável quando fila vazia) -->
      <button
        @click="handleAcceptNext"
        :disabled="queue.length === 0 || isAccepting"
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
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  queue: {
    type: Array,
    default: () => []
  },
  isAccepting: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['acceptTicket'])

function handleAcceptNext() {
  if (props.queue.length > 0 && !props.isAccepting) {
    emit('acceptTicket', props.queue[0])
  }
}
</script>
