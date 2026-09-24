<template>
  <header class="bg-slate-800/80 backdrop-blur border-b border-slate-700/60 px-5 py-3 flex items-center justify-between shadow-sm select-none">
    <!-- Identificação da Estação e Usuário -->
    <div class="flex items-center space-x-4">
      <img
        :src="appIcon"
        alt="Zyron Support"
        class="w-10 h-10 rounded-xl shadow-md shadow-indigo-500/25 border border-indigo-500/30 object-cover"
      />
      <div>
        <div class="flex items-center space-x-2">
          <span class="font-semibold text-slate-100 text-sm tracking-wide">{{ displayName || username || 'Carregando usuário...' }}</span>
          <!-- Função exibida apenas se for cliente comum (não exibida para operador) -->
          <span
            v-if="!isOperator"
            class="bg-emerald-500/20 text-emerald-300 border-emerald-500/30 text-[11px] font-medium px-2 py-0.5 rounded-full border"
          >
            Cliente Comum
          </span>
        </div>
      </div>
    </div>

    <!-- Status da Conexão (Somente Leitura) -->
    <div class="flex items-center space-x-3">
      <!-- Status Badge com Tooltip Informativo de Leitura -->
      <div
        class="flex items-center space-x-2 bg-slate-900/60 px-3 py-1.5 rounded-lg border border-slate-700/50 text-xs cursor-default"
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
    </div>
  </header>
</template>

<script setup>
import { computed } from 'vue'
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
  serverUrl: String
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
