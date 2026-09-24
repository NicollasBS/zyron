<template>
  <!-- Elemento de Áudio Remoto Permanente (Ativo no DOM e Layout Tree do Chromium) -->
  <audio
    ref="clientAudioRef"
    id="remoteOperatorAudio"
    autoplay
    playsinline
    style="position: fixed; top: -9999px; left: -9999px; width: 1px; height: 1px; opacity: 0.01; pointer-events: none;"
  ></audio>

  <!-- MODO FLUTUANTE ESTILO GOOGLE MEET (Quando o cliente está em atendimento com tela compartilhada) -->
  <FloatingCallWidget
    v-if="isFloatingMode && clientTicketStatus === 'in_call' && !isOperator"
    :operator-name="activeOperatorName || activeOperator"
    :duration-seconds="callDuration"
    :is-muted="isMuted"
    :audio-stream="remoteAudioStream"
    @toggle-mute="handleToggleMute"
    @close-session="handleCloseSession"
    @expand-window="handleExpandWindow"
  />

  <div v-else class="h-screen w-screen flex flex-col bg-slate-900 text-slate-100 overflow-hidden select-none">
    <!-- Header com Identificação do Usuário e Status -->
    <HeaderUser
      :username="systemInfo.username"
      :display-name="userDisplayName"
      :hostname="systemInfo.hostname"
      :ip="systemInfo.ip"
      :is-operator="isOperator"
      :connection-status="wsStatus"
      :server-url="serverUrl"
      :app-version="displayVersion"
      :update-available="updateAvailable"
      :latest-version="displayLatestVersion"
      @open-update="showUpdateModal = true"
    />

    <!-- Notificação de Erro Global (se houver) -->
    <div
      v-if="errorMessage"
      class="bg-rose-500/10 border-b border-rose-500/30 px-5 py-2 text-rose-300 text-xs flex items-center justify-between"
    >
      <div class="flex items-center space-x-2">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-rose-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
        <span>{{ errorMessage }}</span>
      </div>
      <button @click="errorMessage = ''" class="text-rose-400 hover:text-white">✕</button>
    </div>

    <!-- Conteúdo Principal -->
    <main class="flex-1 overflow-hidden relative">
      <!-- MODO OPERADOR -->
      <template v-if="isOperator">
        <!-- Operador em Atendimento: Visualizador de Tela e Áudio -->
        <ScreenViewer
          v-if="operatorInCall"
          :client-user="activeClient"
          :client-name="activeClientName"
          :client-ip="activeClientIp"
          :duration-seconds="callDuration"
          :video-stream="remoteVideoStream"
          :audio-stream="remoteAudioStream"
          :app-version="displayVersion"
          @close-session="handleCloseSession"
        >
          <template #voiceControls>
            <VoiceControls
              :is-muted="isMuted"
              :volume="speakerVolume"
              @toggle-mute="handleToggleMute"
              @update-volume="handleUpdateVolume"
            />
          </template>
        </ScreenViewer>

        <!-- Operador Ocioso: Botão Atender Próximo -->
        <OperatorQueueTable
          v-else
          :queue="operatorQueue"
          :is-accepting="isAcceptingTicket"
          @accept-ticket="handleAcceptTicket"
        />
      </template>

      <!-- MODO CLIENTE SOLICITANTE -->
      <template v-else>
        <ClientWaitingQueue
          :ticket-status="clientTicketStatus"
          :position="clientQueuePosition"
          :ticket-id="clientTicketId"
          :operator-user="activeOperator"
          :operator-name="activeOperatorName"
          :call-duration-seconds="callDuration"
          :is-muted="isMuted"
          :is-connecting="wsStatus === 'connecting'"
          :is-awaiting-screen="isAwaitingScreenPermission"
          :audio-stream="remoteAudioStream"
          @request-support="handleRequestSupport"
          @cancel-ticket="handleCancelTicket"
          @close-session="handleCloseSession"
          @toggle-mute="handleToggleMute"
          @minimize-window="handleMinimizeWindow"
          @compact-window="handleCompactWindow"
        />
      </template>
    </main>

    <!-- Modal de Avaliação Obrigatória do Atendimento (Para o cliente ao final da sessão) -->
    <EvaluationModal
      :show="showEvaluationModal"
      :operator-name="lastEvaluatedOperator"
      :ticket-id="lastEvaluatedTicketId"
      @submit-rating="handleSubmitRating"
    />

    <!-- Modal de Atualização Automática do Zyron Client -->
    <UpdateModal
      :show="showUpdateModal"
      :current-version="displayVersion"
      :latest-version="displayLatestVersion"
      :download-url="downloadUrl"
      :release-notes="releaseNotes"
      :sha256="updateSha256"
      :server-url="serverUrl"
      @close="showUpdateModal = false"
      @updated="handleAppUpdated"
    />

    <!-- Indicador de Versão no Canto Inferior Direito da Aplicação -->
    <footer
      v-if="!operatorInCall"
      class="fixed bottom-3 right-4 z-20 flex items-center space-x-2 select-none pointer-events-auto"
    >
      <div
        class="bg-slate-800/80 hover:bg-slate-800 border border-slate-700/60 hover:border-slate-600 px-2.5 py-1 rounded-md text-[11px] font-mono text-slate-400 hover:text-slate-200 transition shadow-sm cursor-default flex items-center space-x-1.5 backdrop-blur-sm"
        :title="`Versão da Aplicação: ${displayVersion}`"
      >
        <span class="w-1.5 h-1.5 rounded-full bg-slate-500"></span>
        <span>{{ displayVersion }}</span>
      </div>

      <!-- Botão para acionar atualização se houver versão mais nova -->
      <button
        v-if="updateAvailable"
        @click.stop="showUpdateModal = true"
        type="button"
        style="--wails-draggable: no-drag;"
        :title="`Nova versão ${displayLatestVersion} disponível! Clique para atualizar.`"
        class="bg-indigo-500/25 hover:bg-indigo-500/40 text-indigo-300 border border-indigo-500/40 text-[11px] font-semibold px-2.5 py-1 rounded-md flex items-center space-x-1.5 transition animate-pulse cursor-pointer shadow-sm"
      >
        <span class="w-1.5 h-1.5 rounded-full bg-cyan-400"></span>
        <span>Atualizar para {{ displayLatestVersion }}</span>
      </button>
    </footer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import HeaderUser from './components/HeaderUser.vue'
import ClientWaitingQueue from './components/ClientWaitingQueue.vue'
import OperatorQueueTable from './components/OperatorQueueTable.vue'
import ScreenViewer from './components/ScreenViewer.vue'
import VoiceControls from './components/VoiceControls.vue'
import EvaluationModal from './components/EvaluationModal.vue'
import FloatingCallWidget from './components/FloatingCallWidget.vue'
import UpdateModal from './components/UpdateModal.vue'
import { wsService } from './services/ws'
import { webrtcService } from './services/webrtc'
import { GetSystemInfo, GetConfig, CheckProfile, MinimizeWindow, RestoreAndFocusWindow, SetFloatingMode, GetAppVersion, CheckForUpdate } from '../wailsjs/go/main/App'

// Estado da Estação e Configurações
const systemInfo = ref({
  username: '',
  hostname: '',
  ip: ''
})
const serverUrl = ref('http://localhost:8080')
const isOperator = ref(false)
const wsStatus = ref('disconnected')
const errorMessage = ref('')
const userDisplayName = ref('')

// Estado do Cliente Solicitante
const clientTicketStatus = ref('idle') // 'idle' | 'waiting' | 'in_call'
const clientQueuePosition = ref(0)
const clientTicketId = ref('')
const activeOperator = ref('')
const activeOperatorName = ref('')
const clientAudioRef = ref(null)
const isAwaitingScreenPermission = ref(false)
const isFloatingMode = ref(false)
const showEvaluationModal = ref(false)
const lastEvaluatedTicketId = ref('')
const lastEvaluatedOperator = ref('')

// Estado de Atualização Automática e Versão
const appVersion = ref('1.0.0')
const updateAvailable = ref(false)
const latestVersion = ref('')
const downloadUrl = ref('')
const releaseNotes = ref('')
const updateSha256 = ref('')
const showUpdateModal = ref(false)

const displayVersion = computed(() => {
  const v = appVersion.value || '1.0.0'
  return v.startsWith('v') || v.startsWith('V') ? v : `v${v}`
})

const displayLatestVersion = computed(() => {
  const v = latestVersion.value || ''
  if (!v) return ''
  return v.startsWith('v') || v.startsWith('V') ? v : `v${v}`
})

// Estado do Operador
const operatorQueue = ref([])
const operatorInCall = ref(false)
const activeClient = ref('')
const activeClientName = ref('')
const activeClientIp = ref('')
const activeTicketId = ref('')
const isAcceptingTicket = ref(false)
const remoteVideoStream = ref(null)
const remoteAudioStream = ref(null)
const speakerVolume = ref(1.0)

// Estado da Chamada / Sessão
const isMuted = ref(false)
const callDuration = ref(0)
let callTimer = null

function startCallTimer() {
  stopCallTimer()
  callDuration.value = 0
  callTimer = setInterval(() => {
    callDuration.value++
  }, 1000)
}

function stopCallTimer() {
  if (callTimer) {
    clearInterval(callTimer)
    callTimer = null
  }
}

// Player nativo de áudio para fallback garantido caso o DOM seja suspenso
let permanentAudioPlayer = null
let clientAudioContext = null

function playRemoteAudioForClient(stream) {
  if (!stream) return
  console.log('[App] Reproduzindo áudio do operador para o cliente:', stream.id, 'Tracks:', stream.getAudioTracks().length)

  // 1. Toca no elemento do template
  if (clientAudioRef.value) {
    if (clientAudioRef.value.srcObject !== stream) {
      clientAudioRef.value.srcObject = stream
    }
    clientAudioRef.value.volume = 1.0
    clientAudioRef.value.muted = false
    const playPromise = clientAudioRef.value.play()
    if (playPromise !== undefined) {
      playPromise.catch((err) => {
        console.warn('[App] clientAudioRef.play() rejeitado, iniciando fallback nativo:', err)
        startNativeAudioFallback(stream)
      })
    }
  } else {
    startNativeAudioFallback(stream)
  }

  // 2. Conecta ao Web Audio API (AudioContext) como garantia em nível de sistema
  startAudioContextFallback(stream)
}

function startNativeAudioFallback(stream) {
  try {
    if (!permanentAudioPlayer) {
      permanentAudioPlayer = new Audio()
      permanentAudioPlayer.autoplay = true
    }
    if (permanentAudioPlayer.srcObject !== stream) {
      permanentAudioPlayer.srcObject = stream
    }
    permanentAudioPlayer.volume = 1.0
    permanentAudioPlayer.muted = false
    permanentAudioPlayer.play().catch((err) => {
      console.warn('[App] Fallback nativo Audio() também falhou:', err)
    })
  } catch (e) {
    console.warn('[App] Falha ao configurar fallback nativo de áudio:', e)
  }
}

function startAudioContextFallback(stream) {
  try {
    const AudioCtx = window.AudioContext || window.webkitAudioContext
    if (!AudioCtx) return
    if (!clientAudioContext || clientAudioContext.state === 'closed') {
      clientAudioContext = new AudioCtx()
    }
    if (clientAudioContext.state === 'suspended') {
      clientAudioContext.resume().catch(console.warn)
    }
    const source = clientAudioContext.createMediaStreamSource(stream)
    source.connect(clientAudioContext.destination)
    console.log('[App] Stream de áudio do operador conectado com sucesso ao Web AudioContext!')
  } catch (err) {
    console.warn('[App] Web AudioContext notice:', err)
  }
}

/**
 * Inicialização do Wails Desktop e verificação de perfil
 */
async function initializeApp() {
  try {
    // 1. Obtenção de dados do sistema via backend Go (Wails Bind)
    if (window.go && window.go.main && window.go.main.App) {
      try {
        if (typeof GetAppVersion === 'function') {
          const ver = await GetAppVersion()
          if (ver) appVersion.value = ver
        } else if (window.go.main.App.GetAppVersion) {
          const ver = await window.go.main.App.GetAppVersion()
          if (ver) appVersion.value = ver
        }
      } catch (e) {
        console.warn('Erro ao obter versão inicial do app:', e)
      }

      const sys = await GetSystemInfo()
      systemInfo.value = sys

      const cfg = await GetConfig()
      if (cfg && cfg.server_url) {
        serverUrl.value = cfg.server_url
      }

      // 2. Consulta de Perfil no Servidor Central
      const profile = await CheckProfile(serverUrl.value, sys.username)
      if (profile.success) {
        isOperator.value = profile.is_operator
        userDisplayName.value = profile.display_name || sys.username
      } else {
        userDisplayName.value = sys.username
        errorMessage.value = `Não foi possível verificar o perfil no servidor: ${profile.error}`
      }
    } else {
      // Fallback para desenvolvimento em navegador comum (Vite dev server)
      console.warn('Wails bindings não encontrados. Usando perfil padrão de teste.')
      systemInfo.value = {
        username: 'CORP\\185033',
        hostname: 'DEV-WORKSTATION',
        ip: '127.0.0.1'
      }
      userDisplayName.value = '185033'
      isOperator.value = false
      appVersion.value = '1.0.0'
    }

    // 3. Conexão WebSocket para sinalização e presença
    setupWebSocket()

    // 4. Verificação de Atualização Automática
    checkAppUpdate()
  } catch (err) {
    console.error('Erro na inicialização:', err)
    errorMessage.value = `Erro de inicialização: ${err.message}`
  }
}

async function checkAppUpdate() {
  try {
    if (typeof GetAppVersion === 'function') {
      const ver = await GetAppVersion()
      if (ver) appVersion.value = ver
    } else if (window.go?.main?.App?.GetAppVersion) {
      const ver = await window.go.main.App.GetAppVersion()
      if (ver) appVersion.value = ver
    }

    let res = null
    if (typeof CheckForUpdate === 'function') {
      res = await CheckForUpdate(serverUrl.value)
    } else if (window.go?.main?.App?.CheckForUpdate) {
      res = await window.go.main.App.CheckForUpdate(serverUrl.value)
    }

    if (res && res.update_available) {
      updateAvailable.value = true
      latestVersion.value = res.latest_version
      downloadUrl.value = res.download_url
      releaseNotes.value = res.release_notes
      updateSha256.value = res.sha256
      showUpdateModal.value = true
    }
  } catch (e) {
    console.warn('Erro ao verificar atualizações do Zyron:', e)
  }
}

function handleAppUpdated() {
  console.log('[App] Atualização aplicada. O aplicativo reiniciará em instantes.')
}

/**
 * Configuração e Listeners do WebSocket
 */
function setupWebSocket() {
  // Limpa listeners antigos para evitar duplicidade
  wsService.removeAllListeners()

  wsService.connect(serverUrl.value, systemInfo.value.username, isOperator.value)

  wsService.on('status', ({ status, error }) => {
    wsStatus.value = status
    if (error) {
      console.warn('Status WS:', status, error)
    }
  })

  // Mensagens do Cliente Solicitante
  wsService.on('ticket_created', (msg) => {
    const p = msg.payload || {}
    clientTicketId.value = p.ticket_id || p.id || ''
    clientQueuePosition.value = p.position || p.queue_position || 1
    clientTicketStatus.value = 'waiting'
  })

  wsService.on('queue_position', (msg) => {
    const p = msg.payload || {}
    if (p.status === 'not_in_queue') {
      clientTicketStatus.value = 'idle'
      clientQueuePosition.value = 0
    } else {
      clientQueuePosition.value = p.position || p.queue_position || 1
      clientTicketStatus.value = 'waiting'
    }
  })

  wsService.on('ticket_cancelled', () => {
    clientTicketStatus.value = 'idle'
    clientQueuePosition.value = 0
    clientTicketId.value = ''
  })

  // Mensagens do Operador de Suporte
  wsService.on('queue_update', (msg) => {
    if (Array.isArray(msg.payload)) {
      operatorQueue.value = msg.payload
      if (isAcceptingTicket.value && msg.payload.length === 0) {
        isAcceptingTicket.value = false
      }
    }
  })

  // Falha ao aceitar chamado (ex: corrida com outro atendente ou chamado já atendido)
  wsService.on('ticket_accept_failed', (msg) => {
    isAcceptingTicket.value = false
    const p = msg.payload || {}
    errorMessage.value = p.error || 'Chamado já foi atendido por outra pessoa.'
    setTimeout(() => {
      if (errorMessage.value === (p.error || 'Chamado já foi atendido por outra pessoa.')) {
        errorMessage.value = ''
      }
    }, 5000)
  })

  // Aceite do Chamado (Recebido por ambas as pontas)
  wsService.on('ticket_accepted', async (msg) => {
    console.log('[App] Chamado aceito:', msg)
    const p = msg.payload || {}

    if (!isOperator.value) {
      // CLIENTE SOLICITANTE: O operador aceitou o chamado.
      // 1. Restaura e foca a janela para o cliente autorizar a tela cheia
      try {
        if (window.go?.main?.App?.RestoreAndFocusWindow) {
          await RestoreAndFocusWindow()
        }
      } catch (e) {
        console.warn('Erro ao restaurar janela:', e)
      }

      activeOperator.value = p.operator || msg.from || ''
      activeOperatorName.value = p.operator_name || p.operator || msg.from || ''
      clientTicketId.value = p.ticket_id || clientTicketId.value
      clientTicketStatus.value = 'in_call'
      isAwaitingScreenPermission.value = true
      startCallTimer()

      try {
        const offer = await webrtcService.startCaller({
          onIceCandidate: (candidate) => {
            const candData = candidate?.toJSON ? candidate.toJSON() : candidate
            wsService.sendCandidate(activeOperator.value, candData)
          },
          onRemoteAudio: (audioStream) => {
            remoteAudioStream.value = audioStream
            playRemoteAudioForClient(audioStream)
          },
          onScreenStop: () => {
            handleCloseSession()
          },
          onPromptRetry: (warnMsg) => {
            errorMessage.value = warnMsg
          }
        })

        if (!offer) {
          isAwaitingScreenPermission.value = false
          return
        }

        isAwaitingScreenPermission.value = false
        errorMessage.value = ''

        // 2. Chamada em andamento: ativa modo flutuante compacto e fixo sobre as outras telas
        isFloatingMode.value = true
        try {
          if (window.go?.main?.App?.SetFloatingMode) {
            await SetFloatingMode(true)
          }
        } catch (e) {
          console.warn('Erro ao ativar modo flutuante:', e)
        }

        // Envia Offer SDP para o operador
        const offerData = offer?.toJSON ? offer.toJSON() : offer
        wsService.sendOffer(activeOperator.value, offerData)
      } catch (err) {
        isAwaitingScreenPermission.value = false
        if (webrtcService.isAbortingCaller) {
          return
        }
        console.error('Falha ao iniciar compartilhamento de mídia:', err)
        errorMessage.value = `Erro ao iniciar mídia: ${err.message}`
        handleCloseSession()
      }
    } else {
      // OPERADOR: Servidor confirmou que você atendeu o chamado. Iniciar WebRTC receiver!
      activeClient.value = p.client_user || ''
      activeClientName.value = p.client_name || p.client_user || activeClientName.value || ''
      activeClientIp.value = p.client_ip || ''
      activeTicketId.value = p.ticket_id || ''
      operatorInCall.value = true
      isAcceptingTicket.value = false
      startCallTimer()

      try {
        await webrtcService.startReceiver({
          onIceCandidate: (candidate) => {
            const candData = candidate?.toJSON ? candidate.toJSON() : candidate
            wsService.sendCandidate(activeClient.value, candData)
          },
          onRemoteTrack: (kind, stream) => {
            if (kind === 'video') {
              remoteVideoStream.value = stream
            } else if (kind === 'audio') {
              remoteAudioStream.value = stream
            }
          }
        })
      } catch (err) {
        console.error('Erro ao iniciar receptor no operador:', err)
      }
    }
  })

  // Sinalização WebRTC: Oferta recebida pelo Operador
  wsService.on('offer', async (msg) => {
    if (isOperator.value) {
      try {
        const answer = await webrtcService.handleOffer(msg.payload)
        const answerData = answer?.toJSON ? answer.toJSON() : answer
        wsService.sendAnswer(msg.from, answerData)
      } catch (err) {
        console.error('Erro ao responder oferta WebRTC:', err)
      }
    }
  })

  // Sinalização WebRTC: Resposta (Answer) recebida pelo Cliente
  wsService.on('answer', async (msg) => {
    if (!isOperator.value) {
      try {
        await webrtcService.handleAnswer(msg.payload)
      } catch (err) {
        console.error('Erro ao aplicar answer WebRTC:', err)
      }
    }
  })

  // Sinalização WebRTC: ICE Candidate recebido
  wsService.on('candidate', async (msg) => {
    try {
      await webrtcService.handleCandidate(msg.payload)
    } catch (err) {
      console.error('Erro ao adicionar ICE candidate:', err)
    }
  })

  // Encerramento de Sessão
  wsService.on('session_closed', () => {
    finishActiveSession('A sessão de atendimento foi encerrada.')
  })

  wsService.on('peer_disconnected', (msg) => {
    finishActiveSession(`A outra ponta (${msg.from || 'remota'}) desconectou da sessão.`)
  })
}

function finishActiveSession(noticeMsg) {
  stopCallTimer()
  webrtcService.close()

  if (isFloatingMode.value) {
    isFloatingMode.value = false
    try {
      if (window.go?.main?.App?.SetFloatingMode) {
        SetFloatingMode(false)
      }
    } catch (e) {
      console.warn('Erro ao restaurar modo normal:', e)
    }
  }

  const wasInCall = clientTicketStatus.value === 'in_call'
  const evaluatedTicketId = clientTicketId.value || activeTicketId.value
  const evaluatedOperator = activeOperatorName.value || activeOperator.value

  if (isOperator.value) {
    operatorInCall.value = false
    activeClient.value = ''
    activeClientName.value = ''
    activeClientIp.value = ''
    activeTicketId.value = ''
    remoteVideoStream.value = null
    remoteAudioStream.value = null
  } else {
    clientTicketStatus.value = 'idle'
    clientQueuePosition.value = 0
    clientTicketId.value = ''
    activeOperator.value = ''
    activeOperatorName.value = ''
    if (clientAudioRef.value) {
      try {
        clientAudioRef.value.pause()
        clientAudioRef.value.srcObject = null
      } catch (e) {}
    }
    if (permanentAudioPlayer) {
      try {
        permanentAudioPlayer.pause()
        permanentAudioPlayer.srcObject = null
      } catch (e) {}
      permanentAudioPlayer = null
    }
    if (clientAudioContext) {
      try {
        clientAudioContext.close()
      } catch (e) {}
      clientAudioContext = null
    }
    remoteAudioStream.value = null

    // Se o cliente solicitante estava em atendimento, exibe a modal de avaliação
    if (wasInCall && evaluatedTicketId) {
      lastEvaluatedTicketId.value = evaluatedTicketId
      lastEvaluatedOperator.value = evaluatedOperator
      showEvaluationModal.value = true
      try {
        if (window.go?.main?.App?.RestoreAndFocusWindow) {
          RestoreAndFocusWindow()
        }
      } catch (e) {
        console.warn(e)
      }
    }
  }

  isMuted.value = false
  isAwaitingScreenPermission.value = false
  if (noticeMsg) {
    console.log(noticeMsg)
  }
}

// Ações de Janela Wails (Minimizar, Modo Flutuante e Restaurar)
async function handleMinimizeWindow() {
  try {
    if (window.go?.main?.App?.MinimizeWindow) {
      await MinimizeWindow()
    }
  } catch (e) {
    console.warn('Erro ao minimizar janela:', e)
  }
}

async function handleCompactWindow() {
  isFloatingMode.value = true
  try {
    if (window.go?.main?.App?.SetFloatingMode) {
      await SetFloatingMode(true)
    }
  } catch (e) {
    console.warn('Erro ao compactar janela:', e)
  }
}

async function handleExpandWindow() {
  isFloatingMode.value = false
  try {
    if (window.go?.main?.App?.SetFloatingMode) {
      await SetFloatingMode(false)
    }
  } catch (e) {
    console.warn('Erro ao expandir janela:', e)
  }
}

async function handleSubmitRating(data) {
  // Envia via WebSocket
  wsService.send('rate_session', '', {
    ticket_id: data.ticketId,
    rating: data.rating,
    feedback: data.feedback
  })

  // Envia também via HTTP REST para garantir persistência mesmo se desconectado
  try {
    await fetch(`${serverUrl.value}/api/session/rate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ticket_id: data.ticketId,
        client_username: systemInfo.value.username,
        rating: data.rating,
        feedback: data.feedback
      })
    })
  } catch (e) {
    console.warn('Erro ao enviar avaliação via HTTP:', e)
  }

  showEvaluationModal.value = false
}

// Ações do Usuário Solicitante
function handleRequestSupport() {
  if (wsStatus.value !== 'connected') {
    errorMessage.value = 'Servidor de sinalização desconectado. Verifique a URL do servidor.'
    return
  }
  errorMessage.value = ''
  wsService.requestSupport(systemInfo.value.hostname, systemInfo.value.ip, userDisplayName.value)
}

function handleCancelTicket() {
  wsService.cancelSupport()
  clientTicketStatus.value = 'idle'
}

// Ações do Operador
function handleAcceptTicket(ticket) {
  isAcceptingTicket.value = true
  const ticketId = ticket.id || ticket.ticket_id
  const clientUser = ticket.client_user
  activeClientName.value = ticket.client_name || ticket.client_user || ''
  wsService.acceptTicket(ticketId, clientUser)
}

function handleRefreshQueue() {
  wsService.send('get_queue', '')
}

// Encerramento de Sessão
function handleCloseSession() {
  const ticketId = isOperator.value ? activeTicketId.value : clientTicketId.value
  const targetUser = isOperator.value ? activeClient.value : activeOperator.value

  wsService.closeSession(ticketId, targetUser)
  finishActiveSession('Sessão encerrada.')
}

// Controles de Mídia
function handleToggleMute() {
  isMuted.value = webrtcService.toggleMute()
}

function handleUpdateVolume(vol) {
  speakerVolume.value = vol
  const clientAudio = document.getElementById('clientVoiceAudio')
  if (clientAudio) {
    clientAudio.volume = vol
  }
}

onMounted(() => {
  initializeApp()
})

onUnmounted(() => {
  finishActiveSession()
  wsService.disconnect()
})
</script>
