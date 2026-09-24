<template>
  <div
    v-if="show"
    class="fixed inset-0 z-50 bg-black/80 backdrop-blur-md flex items-center justify-center p-4 select-none animate-fadeIn"
    style="--wails-draggable: no-drag;"
  >
    <div
      class="bg-slate-800 border border-indigo-500/30 rounded-3xl w-full max-w-md p-7 shadow-2xl text-center relative overflow-hidden"
    >
      <!-- Glow Decorativo -->
      <div class="absolute -top-12 -right-12 w-36 h-36 bg-indigo-500/15 rounded-full blur-2xl pointer-events-none"></div>

      <!-- Ícone Central -->
      <div
        class="w-16 h-16 rounded-2xl mx-auto flex items-center justify-center mb-4 shadow-inner"
        :class="status === 'error' ? 'bg-rose-500/20 border border-rose-500/30 text-rose-400' : 'bg-indigo-500/20 border border-indigo-500/30 text-indigo-400'"
      >
        <!-- Ícone Erro -->
        <svg v-if="status === 'error'" xmlns="http://www.w3.org/2000/svg" class="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
        <!-- Ícone Spinner Baixando -->
        <svg v-else-if="status === 'downloading' || status === 'restarting'" class="animate-spin w-8 h-8" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
        <!-- Ícone Sucesso ou Pronto -->
        <svg v-else xmlns="http://www.w3.org/2000/svg" class="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
        </svg>
      </div>

      <!-- Tag de Versão -->
      <div class="inline-flex items-center space-x-1.5 bg-indigo-500/20 text-indigo-300 border border-indigo-500/30 px-3 py-0.5 rounded-full text-xs font-semibold mb-3">
        <span>Nova Versão {{ displayLatestVersion }}</span>
      </div>

      <h3 class="text-xl font-bold text-white mb-2">
        <span v-if="status === 'idle'">Atualização Disponível</span>
        <span v-else-if="status === 'downloading'">Atualizando Aplicativo...</span>
        <span v-else-if="status === 'restarting'">Reiniciando Aplicação...</span>
        <span v-else>Falha na Atualização</span>
      </h3>

      <p class="text-xs text-slate-300 mb-5 leading-relaxed max-w-sm mx-auto">
        <span v-if="status === 'idle'">
          Uma versão mais recente do Zyron Client está disponível no servidor central ({{ displayCurrentVersion }} ➔ {{ displayLatestVersion }}).
        </span>
        <span v-else-if="status === 'downloading'">
          Baixando nova versão ({{ displayCurrentVersion }} ➔ {{ displayLatestVersion }}) do servidor central. O aplicativo reiniciará automaticamente em instantes.
        </span>
        <span v-else-if="status === 'restarting'">
          O arquivo foi verificado. A aplicação está sendo reiniciada automaticamente com a nova versão...
        </span>
        <span v-else class="text-rose-300 font-medium">
          {{ errorMessage || 'Não foi possível completar a atualização.' }}
        </span>
      </p>

      <!-- Notas da Versão (se houver e em estado idle) -->
      <div
        v-if="status === 'idle' && releaseNotes"
        class="bg-slate-900/60 border border-slate-700/60 rounded-xl p-3 mb-6 text-left max-h-28 overflow-y-auto"
      >
        <div class="text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-1">Novidades</div>
        <p class="text-xs text-slate-300 whitespace-pre-line">{{ releaseNotes }}</p>
      </div>

      <!-- Barra de Progresso Animada -->
      <div v-if="status === 'downloading'" class="w-full bg-slate-900/80 rounded-full h-2 mb-6 overflow-hidden border border-slate-700">
        <div class="bg-gradient-to-r from-indigo-500 to-cyan-400 h-2 rounded-full animate-pulse w-full"></div>
      </div>

      <!-- Ações do Modal -->
      <div class="flex items-center space-x-3">
        <!-- Em estado Pronto -->
        <template v-if="status === 'idle'">
          <button
            @click="$emit('close')"
            type="button"
            class="flex-1 py-3 px-4 rounded-xl bg-slate-700/80 hover:bg-slate-700 text-slate-300 text-sm font-semibold transition"
          >
            Lembrar depois
          </button>
          <button
            @click="startUpdate"
            type="button"
            class="flex-1 py-3 px-4 rounded-xl bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white text-sm font-semibold shadow-lg shadow-indigo-600/30 transition flex items-center justify-center space-x-1.5 active:scale-95"
          >
            <span>Atualizar Agora</span>
          </button>
        </template>

        <!-- Em estado de Erro -->
        <template v-else-if="status === 'error'">
          <button
            @click="$emit('close')"
            type="button"
            class="flex-1 py-3 px-4 rounded-xl bg-slate-700/80 hover:bg-slate-700 text-slate-300 text-sm font-semibold transition"
          >
            Fechar
          </button>
          <button
            @click="startUpdate"
            type="button"
            class="flex-1 py-3 px-4 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-semibold transition active:scale-95"
          >
            Tentar Novamente
          </button>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { ApplyUpdate } from '../../wailsjs/go/main/App'

const props = defineProps({
  show: {
    type: Boolean,
    default: false
  },
  currentVersion: {
    type: String,
    default: '1.0.0'
  },
  latestVersion: {
    type: String,
    default: '1.0.1'
  },
  downloadUrl: {
    type: String,
    default: ''
  },
  releaseNotes: {
    type: String,
    default: ''
  },
  sha256: {
    type: String,
    default: ''
  },
  serverUrl: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['close', 'updated'])

// Estados: 'idle' | 'downloading' | 'restarting' | 'error'
const status = ref('idle')
const errorMessage = ref('')

const displayCurrentVersion = computed(() => {
  const v = props.currentVersion || '1.0.0'
  return v.startsWith('v') || v.startsWith('V') ? v : `v${v}`
})

const displayLatestVersion = computed(() => {
  const v = props.latestVersion || ''
  return v.startsWith('v') || v.startsWith('V') ? v : `v${v}`
})

watch(() => props.show, (newVal) => {
  if (newVal) {
    errorMessage.value = ''
    startUpdate()
  }
})

async function startUpdate() {
  status.value = 'downloading'
  errorMessage.value = ''

  try {
    let res = null
    if (typeof ApplyUpdate === 'function') {
      res = await ApplyUpdate(
        props.serverUrl,
        props.downloadUrl,
        props.sha256
      )
    } else if (window.go?.main?.App?.ApplyUpdate) {
      res = await window.go.main.App.ApplyUpdate(
        props.serverUrl,
        props.downloadUrl,
        props.sha256
      )
    }

    if (res && res.success) {
      status.value = 'restarting'
      emit('updated')
    } else if (res) {
      status.value = 'error'
      errorMessage.value = res.error || 'Erro ao aplicar a atualização.'
    } else {
      // Fallback em ambiente de desenvolvimento web
      console.warn('Wails bindings não disponíveis no navegador para ApplyUpdate.')
      setTimeout(() => {
        status.value = 'error'
        errorMessage.value = 'Modo de desenvolvimento: executável de produção não acessível via navegador.'
      }, 1500)
    }
  } catch (err) {
    status.value = 'error'
    errorMessage.value = err.message || 'Erro inesperado durante a atualização.'
  }
}
</script>
