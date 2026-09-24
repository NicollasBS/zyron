/**
 * Gerenciador WebRTC P2P (Tela e Voz) para rede corporativa (LAN)
 */
export class WebRTCService {
  constructor() {
    this.peerConnection = null
    this.localScreenStream = null
    this.localAudioStream = null
    this.remoteStream = null
    this.isMuted = false
    this.pendingIceCandidates = []
    this.hasRemoteDescription = false
    this.onIceCandidateCallback = null
    this.onTrackCallback = null
    this.isAbortingCaller = false
    this.receiverReadyPromise = null
  }

  /**
   * Inicializa uma nova instância de RTCPeerConnection
   */
  initPeerConnection(onIceCandidate, onTrack) {
    if (onIceCandidate) this.onIceCandidateCallback = onIceCandidate
    if (onTrack) this.onTrackCallback = onTrack

    // Configuração com fallback STUN público para resolução de IPs diretos (evita anonimização mDNS .local no Chromium)
    const config = {
      iceServers: [
        { urls: 'stun:stun.l.google.com:19302' },
        { urls: 'stun:stun1.l.google.com:19302' }
      ],
      iceCandidatePoolSize: 2
    }

    this.peerConnection = new RTCPeerConnection(config)
    this.hasRemoteDescription = false
    this.pendingIceCandidates = []

    this.peerConnection.onicecandidate = (event) => {
      if (event.candidate && this.onIceCandidateCallback) {
        const candidateData = event.candidate.toJSON ? event.candidate.toJSON() : event.candidate
        this.onIceCandidateCallback(candidateData)
      }
    }

    this.peerConnection.ontrack = (event) => {
      console.log('[WebRTC] Track remoto recebido:', event.track.kind, 'ID:', event.track.id)
      let stream = event.streams && event.streams[0]
      if (!stream) {
        stream = new MediaStream([event.track])
      }

      event.track.onunmute = () => {
        console.log('[WebRTC] Track remoto ativo e desmutado (onunmute):', event.track.kind)
        if (this.onTrackCallback) {
          this.onTrackCallback(event.track.kind, stream, event)
        }
      }

      if (this.onTrackCallback) {
        this.onTrackCallback(event.track.kind, stream, event)
      }
    }

    this.peerConnection.onconnectionstatechange = () => {
      console.log('[WebRTC] Connection state:', this.peerConnection?.connectionState)
    }

    this.peerConnection.oniceconnectionstatechange = () => {
      console.log('[WebRTC] ICE state:', this.peerConnection?.iceConnectionState)
    }
  }

  /**
   * FLUXO CLIENTE SOLICITANTE:
   * Captura Tela + Microfone e gera a Offer SDP
   */
  async startCaller({ onIceCandidate, onRemoteAudio, onScreenStop, onPromptRetry }) {
    this.close()
    this.isAbortingCaller = false

    this.initPeerConnection(onIceCandidate, (kind, stream) => {
      if (kind === 'audio' && onRemoteAudio) {
        onRemoteAudio(stream)
      }
    })

    try {
      // 1. Captura de Tela (Vídeo) priorizando Tela Inteira (displaySurface: 'monitor')
      // Se não for aceito, repete a solicitação até que o aceite seja concluído.
      this.localScreenStream = await this.acquireScreenStream(onPromptRetry)
      if (!this.localScreenStream || this.isAbortingCaller) {
        this.close()
        return null
      }

      // Listener para quando o usuário clicar no botão nativo "Parar compartilhamento"
      if (this.localScreenStream.getVideoTracks()[0]) {
        this.localScreenStream.getVideoTracks()[0].onended = () => {
          console.log('[WebRTC] Compartilhamento de tela interrompido pelo usuário.')
          if (onScreenStop) onScreenStop()
        }
      }

      // 2. Captura de Microfone (Voz Bidirecional)
      try {
        this.localAudioStream = await navigator.mediaDevices.getUserMedia({
          audio: {
            echoCancellation: true,
            noiseSuppression: true,
            autoGainControl: true
          },
          video: false
        })
      } catch (audioErr) {
        console.warn('[WebRTC] Microfone não detectado ou permissão negada. Prosseguindo apenas com vídeo:', audioErr)
      }

      // 3. Adiciona os tracks ao PeerConnection com transceivers explícitos
      if (this.localScreenStream) {
        this.localScreenStream.getVideoTracks().forEach((track) => {
          this.peerConnection.addTrack(track, this.localScreenStream)
        })
      }

      // Configura transceiver de vídeo como 'sendonly' (cliente apenas transmite tela, não recebe vídeo do operador)
      const videoTransceiver = this.peerConnection.getTransceivers().find(
        (t) => (t.sender.track && t.sender.track.kind === 'video')
      )
      if (videoTransceiver) {
        videoTransceiver.direction = 'sendonly'
      }

      if (this.localAudioStream) {
        this.localAudioStream.getAudioTracks().forEach((track) => {
          track.enabled = !this.isMuted
          this.peerConnection.addTrack(track, this.localAudioStream)
        })
      }

      // Garante que o transceiver de áudio esteja configurado para envio e recebimento de áudio do operador
      const audioTransceiver = this.peerConnection.getTransceivers().find(
        (t) => (t.sender.track && t.sender.track.kind === 'audio') || (t.receiver.track && t.receiver.track.kind === 'audio')
      )
      if (audioTransceiver) {
        audioTransceiver.direction = this.localAudioStream ? 'sendrecv' : 'recvonly'
      } else {
        this.peerConnection.addTransceiver('audio', {
          direction: this.localAudioStream ? 'sendrecv' : 'recvonly'
        })
      }

      // 4. Cria a Oferta SDP
      const offer = await this.peerConnection.createOffer()
      await this.peerConnection.setLocalDescription(offer)

      return offer.toJSON ? offer.toJSON() : offer
    } catch (err) {
      if (this.isAbortingCaller) {
        return null
      }
      console.error('[WebRTC] Erro ao iniciar compartilhamento:', err)
      this.close()
      throw err
    }
  }

  /**
   * Solicita captura de tela priorizando Tela Inteira (displaySurface: 'monitor').
   * Se o usuário cancelar ou recusar, solicita repetidamente até o aceite ser feito.
   */
  async acquireScreenStream(onPromptRetry) {
    let retryCount = 0

    while (!this.isAbortingCaller) {
      try {
        console.log('[WebRTC] Solicitando captura de Tela Inteira...')

        let stream = null
        try {
          stream = await navigator.mediaDevices.getDisplayMedia({
            video: {
              cursor: 'always',
              displaySurface: 'monitor',
              frameRate: { ideal: 30, max: 60 }
            },
            audio: false
          })
        } catch (optionsErr) {
          // Fallback caso a versão do WebView2 não aceite displaySurface nas constraints de vídeo
          if (optionsErr && (optionsErr.name === 'TypeError' || optionsErr.name === 'OverconstrainedError')) {
            console.warn('[WebRTC] displaySurface não suportado nas constraints, tentando formato padrão:', optionsErr)
            stream = await navigator.mediaDevices.getDisplayMedia({
              video: {
                cursor: 'always',
                frameRate: { ideal: 30, max: 60 }
              },
              audio: false
            })
          } else {
            throw optionsErr
          }
        }

        if (this.isAbortingCaller) {
          if (stream) {
            stream.getTracks().forEach((t) => t.stop())
          }
          return null
        }

        const videoTrack = stream.getVideoTracks()[0]
        if (videoTrack) {
          const settings = videoTrack.getSettings ? videoTrack.getSettings() : {}
          // Se o usuário selecionou uma janela individual ao invés de tela inteira
          if (settings.displaySurface && settings.displaySurface !== 'monitor') {
            console.warn('[WebRTC] Usuário selecionou janela/aplicativo em vez da Tela Inteira:', settings.displaySurface)
            videoTrack.stop()
            if (onPromptRetry) {
              onPromptRetry('Atenção: Por favor, selecione a "Tela Inteira" na lista para continuar o atendimento.')
            }
            await new Promise((r) => setTimeout(r, 600))
            continue
          }
        }

        console.log('[WebRTC] Compartilhamento de tela inteira autorizado com sucesso.')
        return stream
      } catch (err) {
        if (this.isAbortingCaller) {
          return null
        }

        retryCount++
        console.warn(`[WebRTC] Compartilhamento de tela recusado/cancelado (tentativa ${retryCount}). Solicitando novamente até o aceite...`, err)
        if (onPromptRetry) {
          onPromptRetry('O compartilhamento da tela inteira é obrigatório para o atendimento. Por favor, selecione "Tela Inteira" e clique em Compartilhar.')
        }

        // Aguarda 600ms antes de disparar o prompt novamente para a interface do Windows responder com suavidade
        await new Promise((r) => setTimeout(r, 600))
      }
    }

    return null
  }

  /**
   * FLUXO OPERADOR DE SUPORTE:
   * Prepara microfone do operador e aguarda tracks de tela e voz do cliente
   */
  async startReceiver({ onIceCandidate, onRemoteTrack }) {
    this.close()
    this.isAbortingCaller = false

    this.initPeerConnection(onIceCandidate, (kind, stream) => {
      if (onRemoteTrack) {
        onRemoteTrack(kind, stream)
      }
    })

    // Inicia captura prévia do microfone do operador para estar disponível quando a offer chegar
    this.receiverReadyPromise = this.ensureOperatorAudio()
    await this.receiverReadyPromise
  }

  /**
   * Garante a captura do microfone do operador (sem adicionar ao peerConnection antecipadamente para evitar conflito de m-lines)
   */
  async ensureOperatorAudio() {
    if (this.localAudioStream && this.localAudioStream.getAudioTracks().some((t) => t.readyState === 'live')) {
      return this.localAudioStream
    }

    try {
      this.localAudioStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true
        },
        video: false
      })
      console.log('[WebRTC] Microfone do operador capturado com sucesso:', this.localAudioStream.getAudioTracks()[0]?.label)
      return this.localAudioStream
    } catch (audioErr) {
      console.warn('[WebRTC] Operador sem microfone ou permissão negada:', audioErr)
      return null
    }
  }

  /**
   * Trata a Offer recebida pelo Operador e gera a Answer SDP com microfone do operador ativo
   */
  async handleOffer(offer) {
    // 1. Aguarda a captura do microfone do operador (se ainda estiver em andamento)
    if (this.receiverReadyPromise) {
      try {
        await this.receiverReadyPromise
      } catch (e) {
        console.warn('[WebRTC] Erro ao aguardar inicialização do microfone do operador:', e)
      }
    }
    if (!this.localAudioStream) {
      await this.ensureOperatorAudio()
    }

    if (!this.peerConnection) {
      console.warn('[WebRTC] PeerConnection ainda não estava instanciado ao receber offer. Inicializando receptor...')
      this.initPeerConnection(this.onIceCandidateCallback, this.onTrackCallback)
    }

    // 2. Aplica a descrição remota da oferta do cliente (isto instancia os transceivers alinhados com a oferta)
    await this.peerConnection.setRemoteDescription(new RTCSessionDescription(offer))
    this.hasRemoteDescription = true
    await this.flushPendingCandidates()

    // 3. Associa o microfone do operador ao PeerConnection e garante envio bidirecional
    const audioTrack = this.localAudioStream?.getAudioTracks().find((t) => t.readyState === 'live')
    if (audioTrack) {
      audioTrack.enabled = !this.isMuted
      try {
        this.peerConnection.addTrack(audioTrack, this.localAudioStream)
        console.log('[WebRTC] Microfone do operador vinculado via addTrack:', audioTrack.label)
      } catch (addErr) {
        console.warn('[WebRTC] addTrack notice:', addErr)
      }
    }

    const audioTransceiver = this.peerConnection.getTransceivers().find(
      (t) => (t.receiver?.track && t.receiver.track.kind === 'audio') ||
             (t.sender?.track && t.sender.track.kind === 'audio')
    )

    if (audioTransceiver) {
      if (audioTrack) {
        audioTransceiver.direction = 'sendrecv'
        if (!audioTransceiver.sender.track) {
          await audioTransceiver.sender.replaceTrack(audioTrack)
        }
        console.log('[WebRTC] audioTransceiver configurado como sendrecv com track:', audioTrack.label)
      } else {
        audioTransceiver.direction = 'recvonly'
        console.log('[WebRTC] Sem microfone no operador. Transceiver de áudio configurado como recvonly.')
      }
    }

    // 4. Cria a Answer SDP com áudio bidirecional ativo
    const answer = await this.peerConnection.createAnswer()
    await this.peerConnection.setLocalDescription(answer)

    console.log('[WebRTC] Answer criada pelo operador (áudio ativo:', !!audioTrack, ')')
    return answer.toJSON ? answer.toJSON() : answer
  }

  /**
   * Trata a Answer recebida pelo Cliente Solicitante
   */
  async handleAnswer(answer) {
    if (!this.peerConnection) return
    await this.peerConnection.setRemoteDescription(new RTCSessionDescription(answer))
    this.hasRemoteDescription = true
    await this.flushPendingCandidates()
  }

  /**
   * Trata novos ICE Candidates recebidos via WebSocket
   */
  async handleCandidate(candidate) {
    if (!this.peerConnection || !candidate) return

    // Ignora pacotes de candidato nulos ou vazios
    if (!candidate.candidate && typeof candidate === 'object' && !candidate.sdpMid && candidate.sdpMLineIndex === undefined) {
      return
    }

    if (!this.hasRemoteDescription) {
      this.pendingIceCandidates.push(candidate)
      return
    }

    try {
      await this.peerConnection.addIceCandidate(new RTCIceCandidate(candidate))
    } catch (err) {
      console.error('[WebRTC] Falha ao adicionar ICE Candidate:', err)
    }
  }

  async flushPendingCandidates() {
    while (this.pendingIceCandidates.length > 0) {
      const cand = this.pendingIceCandidates.shift()
      try {
        await this.peerConnection.addIceCandidate(new RTCIceCandidate(cand))
      } catch (err) {
        console.error('[WebRTC] Falha ao processar ICE Candidate pendente:', err)
      }
    }
  }

  /**
   * Alterna mudo do microfone local
   */
  toggleMute() {
    this.isMuted = !this.isMuted
    if (this.localAudioStream) {
      this.localAudioStream.getAudioTracks().forEach((track) => {
        track.enabled = !this.isMuted
      })
    }
    return this.isMuted
  }

  /**
   * Encerra todos os fluxos de mídia e fecha a conexão WebRTC
   */
  close() {
    this.isAbortingCaller = true
    if (this.localScreenStream) {
      this.localScreenStream.getTracks().forEach((track) => {
        try {
          track.stop()
        } catch (e) {}
      })
      this.localScreenStream = null
    }

    if (this.localAudioStream) {
      this.localAudioStream.getTracks().forEach((track) => {
        try {
          track.stop()
        } catch (e) {}
      })
      this.localAudioStream = null
    }

    if (this.peerConnection) {
      try {
        this.peerConnection.close()
      } catch (e) {}
      this.peerConnection = null
    }

    this.pendingIceCandidates = []
    this.hasRemoteDescription = false
    this.isMuted = false
    this.receiverReadyPromise = null
    console.log('[WebRTC] Sessão de mídia encerrada.')
  }
}

export const webrtcService = new WebRTCService()
