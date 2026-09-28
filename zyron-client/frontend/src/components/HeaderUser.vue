<template>
  <header
    style="--wails-draggable: drag;"
    @dblclick="handleToggleMaximize"
    class="bg-slate-800/90 backdrop-blur border-b border-slate-700/60 px-4 py-2.5 flex items-center justify-between shadow-sm select-none"
  >
    <!-- Identificação da Estação e Usuário -->
    <div class="flex items-center space-x-3 pointer-events-auto" style="--wails-draggable: no-drag;">
      <img
        :src="appIcon"
        alt="Zyron Support"
        class="w-8 h-8 rounded-lg shadow-md shadow-indigo-500/25 border border-indigo-500/30 object-cover"
      />
      <div>
        <div class="flex items-center space-x-2">
          <span class="font-semibold text-slate-100 text-xs tracking-wide">{{ displayName || username || 'Carregando usuário...' }}</span>
          <span
            v-if="!isOperator"
            class="bg-emerald-500/20 text-emerald-300 border-emerald-500/30 text-[10px] font-medium px-2 py-0.5 rounded-full border"
          >
            Cliente Comum
          </span>
          <!-- Botão para acionar atualização se houver versão mais nova -->
          <button
            v-if="updateAvailable"
            @click.stop="$emit('openUpdate')"
            type="button"
            style="--wails-draggable: no-drag;"
            title="Nova versão disponível! Clique para atualizar."
            class="bg-indigo-500/25 hover:bg-indigo-500/40 text-indigo-300 border border-indigo-500/40 text-[10px] font-semibold px-2 py-0.5 rounded-full flex items-center space-x-1.5 transition animate-pulse cursor-pointer"
          >
            <span class="w-1.5 h-1.5 rounded-full bg-cyan-400"></span>
            <span>Atualizar para {{ displayLatestVersion }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Área Direita: Status da Conexão e Controles de Janela Imbutidos -->
    <div class="flex items-center space-x-2">
      <!-- Status Badge com Tooltip Informativo de Leitura -->
      <div
        style="--wails-draggable: no-drag;"
        class="flex items-center space-x-2 bg-slate-900/60 px-2.5 py-1 rounded-lg border border-slate-700/50 text-[11px] cursor-default"
        :title="serverUrl ? `Servidor Central: ${serverUrl} (definido via config.json)` : ''"
      >
        <span
          class="w-2 h-2 rounded-full transition-colors duration-300"
          :class="{
            'bg-emerald-500 shadow-sm shadow-emerald-500/50 animate-pulse': connectionStatus === 'connected',
            'bg-amber-400 animate-ping': connectionStatus === 'connecting',
            'bg-rose-500': connectionStatus === 'disconnected' || connectionStatus === 'error'
          }"
        ></span>
        <span class="text-slate-300 capitalize">
          {{ statusLabel }}
        </span>
      </div>

      <!-- Divisor Vertical Sutil -->
      <div class="h-4 w-px bg-slate-700/80 mx-1"></div>

      <!-- Botões Imbutidos de Controle de Janela (Minimizar, Maximizar/Restaurar, Fechar) -->
      <div class="flex items-center space-x-1" style="--wails-draggable: no-drag;">
        <!-- Botão Minimizar -->
        <button
          @click.stop="handleMinimize"
          type="button"
          title="Minimizar"
          class="w-7 h-7 flex items-center justify-center rounded-md text-slate-400 hover:text-white hover:bg-slate-700/70 transition active:scale-90"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M20 12H4" />
          </svg>
        </button>

        <!-- Botão Maximizar / Restaurar -->
        <button
          @click.stop="handleToggleMaximize"
          type="button"
          :title="isMaximized ? 'Restaurar' : 'Maximizar'"
          class="w-7 h-7 flex items-center justify-center rounded-md text-slate-400 hover:text-white hover:bg-slate-700/70 transition active:scale-90"
        >
          <!-- Ícone Restaurar (quando maximizado) -->
          <svg v-if="isMaximized" xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="7" y="7" width="13" height="13" rx="1.5" />
            <path stroke-linecap="round" stroke-linejoin="round" d="M17 7V5.5A1.5 1.5 0 0015.5 4H5.5A1.5 1.5 0 004 5.5V15.5A1.5 1.5 0 005.5 17H7" />
          </svg>
          <!-- Ícone Maximizar (quando normal) -->
          <svg v-else xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="4" y="4" width="16" height="16" rx="2" />
          </svg>
        </button>

        <!-- Botão Fechar -->
        <button
          @click.stop="handleClose"
          type="button"
          title="Fechar"
          class="w-7 h-7 flex items-center justify-center rounded-md text-slate-400 hover:text-white hover:bg-rose-600 transition active:scale-90"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>
  </header>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import appIcon from '../assets/appicon.png'

const props = defineProps({
  username: String,
  displayName: String,
  hostname: String,
  ip: String,
  isOperator: Boolean,
  connectionStatus: {
    type: String,
    default: 'disconnected'
  },
  serverUrl: String,
  appVersion: {
    type: String,
    default: '1.0.0'
  },
  updateAvailable: {
    type: Boolean,
    default: false
  },
  latestVersion: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['openUpdate', 'minimize'])

const isMaximized = ref(false)

async function checkMaximized() {
  try {
    if (window.go?.main?.App?.IsWindowMaximised) {
      isMaximized.value = await window.go.main.App.IsWindowMaximised()
    }
  } catch (e) {
    console.warn('Erro ao checar estado maximizado:', e)
  }
}

async function handleMinimize() {
  emit('minimize')
}

async function handleToggleMaximize() {
  try {
    if (window.go?.main?.App?.ToggleMaximizeWindow) {
      await window.go.main.App.ToggleMaximizeWindow()
    } else if (window.runtime?.WindowToggleMaximise) {
      window.runtime.WindowToggleMaximise()
    }
    // Aguarda atualização e sincroniza estado
    setTimeout(checkMaximized, 80)
  } catch (e) {
    console.warn('Erro ao alternar maximização:', e)
  }
}

async function handleClose() {
  try {
    if (window.go?.main?.App?.CloseWindow) {
      await window.go.main.App.CloseWindow()
    } else if (window.runtime?.Quit) {
      window.runtime.Quit()
    }
  } catch (e) {
    console.warn('Erro ao fechar aplicação:', e)
  }
}

onMounted(() => {
  checkMaximized()
  window.addEventListener('resize', checkMaximized)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMaximized)
})

const displayLatestVersion = computed(() => {
  const v = props.latestVersion || ''
  if (!v) return ''
  return v.startsWith('v') || v.startsWith('V') ? v : `v${v}`
})

const statusLabel = computed(() => {
  switch (props.connectionStatus) {
    case 'connected':
      return 'Conectado'
    case 'connecting':
      return 'Conectando...'
    case 'error':
      return 'Erro Conexão'
    default:
      return 'Desconectado'
  }
})
</script>
