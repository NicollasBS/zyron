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
    this.dataChannel = null
    this.onDataChannelCallback = null
  }

  /**
   * Inicializa uma nova instância de RTCPeerConnection
   */
  initPeerConnection(onIceCandidate, onTrack, onDataChannelMessage) {
    if (onIceCandidate) this.onIceCandidateCallback = onIceCandidate
    if (onTrack) this.onTrackCallback = onTrack
    if (onDataChannelMessage) this.onDataChannelCallback = onDataChannelMessage
    this.dataChannel = null

    // Configuração com fallback STUN público para resolução de IPs diretos (evita anonimização mDNS .local no Chromium)
    const config = {
      iceServers: [
        { urls: 'stun:stun.l.google.com:19302' },
        { urls: 'stun:stun1.l.google.com:19302' }
      ]
    }

    this.peerConnection = new RTCPeerConnection(config)
    this.hasRemoteDescription = false
    this.pendingIceCandidates = []

    this.peerConnection.ondatachannel = (event) => {
      console.log('[WebRTC] DataChannel recebido:', event.channel.label)
      this.setupDataChannel(event.channel)
    }

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

  setupDataChannel(channel) {
    if (!channel) return
    this.dataChannel = channel

    this.dataChannel.onopen = () => {
      console.log('[WebRTC] DataChannel aberto para ponteiro laser:', channel.label)
    }

    this.dataChannel.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        if (this.onDataChannelCallback) {
          this.onDataChannelCallback(data)
        }
      } catch (err) {
        console.warn('[WebRTC] Erro ao decodificar dados do DataChannel:', err)
      }
    }

    this.dataChannel.onclose = () => {
      console.log('[WebRTC] DataChannel fechado')
      if (this.dataChannel === channel) {
        this.dataChannel = null
      }
    }
  }

  createLaserDataChannel() {
    if (!this.peerConnection) return null
    try {
      const channel = this.peerConnection.createDataChannel('zyron-laser', {
        ordered: false,
        maxRetransmits: 0
      })
      this.setupDataChannel(channel)
      return channel
    } catch (err) {
      console.warn('[WebRTC] Falha ao criar DataChannel para laser:', err)
      return null
    }
  }

  sendLaserData(laserData) {
    if (this.dataChannel && this.dataChannel.readyState === 'open') {
      try {
        this.dataChannel.send(JSON.stringify(laserData))
        return true
      } catch (err) {
        console.warn('[WebRTC] Falha ao enviar laser via DataChannel:', err)
      }
    }
    return false
  }

  /**
   * FLUXO CLIENTE SOLICITANTE:
   * Captura Tela + Microfone e gera a Offer SDP
   */
  async startCaller({ onIceCandidate, onRemoteAudio, onScreenStop, onPromptRetry, onLaserData }) {
    this.close()
    this.isAbortingCaller = false

    this.initPeerConnection(onIceCandidate, (kind, stream) => {
      if (kind === 'audio' && onRemoteAudio) {
        onRemoteAudio(stream)
      }
    }, onLaserData)

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

      // 3.1 Cria DataChannel antes da Offer para suporte a telemetria e ponteiro laser em tempo real
      this.createLaserDataChannel()

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
   * Solicita captura de tela ao usuário, priorizando por padrão a Tela Inteira (displaySurface: 'monitor').
   */
  async acquireScreenStream(onPromptRetry) {
    if (this.isAbortingCaller) return null

    try {
      console.log('[WebRTC] Solicitando captura de tela com preferência de Tela Inteira...')

      let stream = null
      try {
        // Solicita priorizando por padrão a aba de Tela Inteira (displaySurface: 'monitor') no diálogo do Windows
        stream = await navigator.mediaDevices.getDisplayMedia({
          video: {
            cursor: 'always',
            displaySurface: 'monitor',
            frameRate: { ideal: 30, max: 60 }
          },
          audio: false,
          selfBrowserSurface: 'exclude',
          surfaceSwitching: 'include',
          monitorTypeSurfaces: 'include',
          preferCurrentTab: false
        })
      } catch (hintErr) {
        // Se o usuário cancelou voluntariamente, propaga o cancelamento sem disparar novamente
        if (hintErr.name === 'NotAllowedError' || hintErr.name === 'AbortError') {
          throw hintErr
        }
        console.warn('[WebRTC] Fallback para constraints padrão de captura:', hintErr)
        stream = await navigator.mediaDevices.getDisplayMedia({
          video: {
            cursor: 'always',
            frameRate: { ideal: 30, max: 60 }
          },
          audio: false
        })
      }

      if (this.isAbortingCaller) {
        if (stream) {
          stream.getTracks().forEach((t) => t.stop())
        }
        return null
      }

      console.log('[WebRTC] Compartilhamento de tela autorizado com sucesso.')
      return stream
    } catch (err) {
      if (this.isAbortingCaller) {
        return null
      }

      console.warn('[WebRTC] Compartilhamento de tela recusado ou cancelado:', err)
      if (onPromptRetry) {
        onPromptRetry('O compartilhamento de tela foi cancelado ou não autorizado.')
      }
      return null
    }
  }

  /**
   * FLUXO OPERADOR DE SUPORTE:
   * Prepara microfone do operador e aguarda tracks de tela e voz do cliente
   */
  async startReceiver({ onIceCandidate, onRemoteTrack, onLaserData }) {
    this.close()
    this.isAbortingCaller = false

    this.initPeerConnection(onIceCandidate, (kind, stream) => {
      if (onRemoteTrack) {
        onRemoteTrack(kind, stream)
      }
    }, onLaserData)

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

    // Ignora candidato se for vazio ou de encerramento
    if (!candidate.candidate || typeof candidate.candidate !== 'string' || candidate.candidate.trim() === '') {
      return
    }

    if (!this.hasRemoteDescription) {
      this.pendingIceCandidates.push(candidate)
      return
    }

    try {
      await this.peerConnection.addIceCandidate(new RTCIceCandidate(candidate))
    } catch (err) {
      console.warn('[WebRTC] Falha ao adicionar ICE Candidate:', err)
    }
  }

  async flushPendingCandidates() {
    while (this.pendingIceCandidates.length > 0) {
      const cand = this.pendingIceCandidates.shift()
      if (!cand || !cand.candidate || typeof cand.candidate !== 'string' || cand.candidate.trim() === '') {
        continue
      }
      try {
        await this.peerConnection.addIceCandidate(new RTCIceCandidate(cand))
      } catch (err) {
        console.warn('[WebRTC] Falha ao processar ICE Candidate pendente:', err)
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

    if (this.dataChannel) {
      try {
        this.dataChannel.close()
      } catch (e) {}
      this.dataChannel = null
    }

    this.pendingIceCandidates = []
    this.hasRemoteDescription = false
    this.isMuted = false
    this.receiverReadyPromise = null
    this.onDataChannelCallback = null
    console.log('[WebRTC] Sessão de mídia encerrada.')
  }
}

export const webrtcService = new WebRTCService()
