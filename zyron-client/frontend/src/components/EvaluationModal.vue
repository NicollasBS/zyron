<template>
  <div
    v-if="show"
    class="fixed inset-0 z-50 bg-black/80 backdrop-blur-md flex items-center justify-center p-4 select-none animate-fadeIn"
  >
    <div
      class="bg-slate-800 border border-slate-700/80 rounded-3xl w-full max-w-md p-7 shadow-2xl text-center relative overflow-hidden"
    >
      <div class="absolute -top-12 -right-12 w-36 h-36 bg-amber-500/10 rounded-full blur-2xl pointer-events-none"></div>

      <!-- Ícone Estrela -->
      <div class="w-16 h-16 rounded-2xl bg-amber-500/15 border border-amber-500/30 text-amber-400 mx-auto flex items-center justify-center mb-3 shadow-inner">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-8 h-8 fill-amber-400" viewBox="0 0 24 24">
          <path d="M12 17.27L18.18 21l-1.64-7.03L22 9.24l-7.19-.61L12 2 9.19 8.63 2 9.24l5.46 4.73L5.82 21z" />
        </svg>
      </div>

      <div class="inline-flex items-center space-x-1.5 bg-amber-500/15 text-amber-300 border border-amber-500/30 px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wider mb-2">
        <span>Avaliação Obrigatória</span>
      </div>

      <h3 class="text-xl font-bold text-white mb-1.5">Avaliação do Atendimento</h3>
      <p class="text-xs text-slate-300 mb-5">
        Como foi sua experiência com <strong class="text-white">{{ operatorName || 'o atendimento' }}</strong>?
      </p>

      <!-- Seletor de 5 Estrelas -->
      <div class="flex items-center justify-center space-x-2 mb-2">
        <button
          v-for="star in 5"
          :key="star"
          type="button"
          @click="selectedRating = star"
          @mouseenter="hoverRating = star"
          @mouseleave="hoverRating = 0"
          class="p-1 transition-transform transform active:scale-90 hover:scale-110 focus:outline-none"
          :title="`${star} de 5 estrelas`"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-9 h-9 transition-colors duration-150 cursor-pointer"
            :class="(hoverRating || selectedRating) >= star ? 'text-amber-400 fill-amber-400 drop-shadow-[0_0_8px_rgba(251,191,36,0.5)]' : 'text-slate-600 fill-transparent'"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="1.5"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
          </svg>
        </button>
      </div>

      <!-- Rótulo da Nota -->
      <div class="text-xs font-semibold h-5 mb-6 tracking-wide" :class="selectedRating > 0 || hoverRating > 0 ? 'text-amber-300' : 'text-slate-400'">
        {{ ratingLabel }}
      </div>

      <!-- Ação Obrigatória de Envio -->
      <div>
        <button
          @click="submitRating"
          :disabled="selectedRating === 0 || isSubmitting"
          class="w-full py-3.5 px-4 rounded-xl bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white font-semibold text-xs shadow-md shadow-indigo-600/30 transition duration-150 flex items-center justify-center space-x-2 disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:from-indigo-600 disabled:hover:to-indigo-500 disabled:shadow-none"
        >
          <svg v-if="isSubmitting" class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          <span>{{ isSubmitting ? 'Enviando...' : (selectedRating === 0 ? 'Selecione uma nota para concluir' : 'Enviar Avaliação') }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'

const props = defineProps({
  show: {
    type: Boolean,
    default: false
  },
  operatorName: {
    type: String,
    default: ''
  },
  ticketId: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['submitRating'])

const selectedRating = ref(0)
const hoverRating = ref(0)
const isSubmitting = ref(false)

watch(() => props.show, (newVal) => {
  if (newVal) {
    selectedRating.value = 0
    hoverRating.value = 0
    isSubmitting.value = false
  }
})

const ratingLabel = computed(() => {
  const current = hoverRating.value || selectedRating.value
  switch (current) {
    case 5:
      return 'Excelente!'
    case 4:
      return 'Muito Bom!'
    case 3:
      return 'Bom'
    case 2:
      return 'Regular'
    case 1:
      return 'Ruim'
    default:
      return 'Por favor, selecione uma nota de 1 a 5 estrelas'
  }
})

function submitRating() {
  if (selectedRating.value === 0 || isSubmitting.value) {
    return
  }
  isSubmitting.value = true
  emit('submitRating', {
    ticketId: props.ticketId,
    rating: selectedRating.value,
    feedback: ''
  })
}
</script>
