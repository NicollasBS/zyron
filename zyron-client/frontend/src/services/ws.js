/**
 * Gerenciador de conexão WebSocket para sinalização com o Zyron Server
 */
class WebSocketService {
  constructor() {
    this.socket = null
    this.serverUrl = ''
    this.username = ''
    this.isOperator = false
    this.listeners = new Map()
    this.reconnectTimer = null
    this.pingTimer = null
    this.isConnected = false
    this.shouldReconnect = true
    this.reconnectAttempts = 0
  }

  /**
   * Conecta ao servidor WebSocket de sinalização
   */
  connect(serverUrl, username, isOperator) {
    if (!serverUrl || !username) {
      console.warn('[WS] URL ou Username não informados para conexão.')
      return
    }

    // Normaliza URL: http(s) -> ws(s)
    let wsBase = serverUrl.replace(/^http:\/\//i, 'ws://').replace(/^https:\/\//i, 'wss://')
    if (!wsBase.startsWith('ws://') && !wsBase.startsWith('wss://')) {
      wsBase = 'ws://' + wsBase
    }
    const wsUrl = `${wsBase.replace(/\/+$/, '')}/ws?username=${encodeURIComponent(username)}&is_operator=${isOperator ? 'true' : 'false'}`

    // Se o socket já estiver ativo ou em processo de conexão com os mesmos dados, evita reconexão redundante
    if (
      this.socket &&
      (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING) &&
      this.serverUrl === serverUrl &&
      this.username === username &&
      this.isOperator === isOperator
    ) {
      console.log('[WS] Conexão WebSocket já ativa ou conectando. Nenhuma ação necessária.')
      return
    }

    this.serverUrl = serverUrl
    this.username = username
    this.isOperator = isOperator
    this.shouldReconnect = true

    // Cancela qualquer timer de reconexão anterior
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }

    // Fecha socket anterior desvinculando os handlers para evitar loop de eventos
    if (this.socket) {
      const oldSocket = this.socket
      this.socket = null
      oldSocket.onopen = null
      oldSocket.onmessage = null
      oldSocket.onerror = null
      oldSocket.onclose = null
      try {
        oldSocket.close(1000, 'Reconnecting')
      } catch (e) {
        // ignore
      }
    }

    console.log('[WS] Conectando a:', wsUrl)
    this.emit('status', { status: 'connecting' })

    try {
      this.socket = new WebSocket(wsUrl)
    } catch (err) {
      console.error('[WS] Erro ao instanciar WebSocket:', err)
      this.emit('status', { status: 'error', error: err.message })
      this.scheduleReconnect()
      return
    }

    const currentSocket = this.socket

    this.socket.onopen = (event) => {
      if (this.socket !== currentSocket) return
      console.log('[WS] Conectado com sucesso')
      this.isConnected = true
      this.reconnectAttempts = 0
      if (this.reconnectTimer) {
        clearTimeout(this.reconnectTimer)
        this.reconnectTimer = null
      }

      this.startPing()
      this.emit('status', { status: 'connected' })
      this.emit('open', event)
    }

    this.socket.onmessage = (event) => {
      if (this.socket !== currentSocket) return
      try {
        const msg = JSON.parse(event.data)
        if (msg.type === 'pong') {
          // Heartbeat pong recebido com sucesso
          return
        }
        console.log('[WS] Mensagem recebida:', msg.type, msg)
        this.emit('message', msg)
        if (msg.type) {
          this.emit(msg.type, msg)
        }
      } catch (err) {
        console.error('[WS] Falha ao processar mensagem JSON:', err, event.data)
      }
    }

    this.socket.onclose = (event) => {
      if (this.socket !== currentSocket) {
        // Ignora eventos de sockets antigos/descartados
        return
      }
      console.warn('[WS] Conexão encerrada:', event.code, event.reason)
      this.stopPing()
      this.isConnected = false
      this.socket = null
      this.emit('status', { status: 'disconnected', code: event.code, reason: event.reason })
      this.emit('close', event)

      if (this.shouldReconnect) {
        this.scheduleReconnect()
      }
    }

    this.socket.onerror = (err) => {
      if (this.socket !== currentSocket) return
      console.error('[WS] Erro no socket:', err)
      this.emit('status', { status: 'error', error: 'Erro de comunicação' })
      this.emit('error', err)
    }
  }

  /**
   * Inicia o envio periódico de pings para manter a conexão e presença ativas no Redis
   */
  startPing() {
    this.stopPing()
    this.pingTimer = setInterval(() => {
      if (this.socket && this.socket.readyState === WebSocket.OPEN) {
        this.send('ping', '')
      }
    }, 15000)
  }

  stopPing() {
    if (this.pingTimer) {
      clearInterval(this.pingTimer)
      this.pingTimer = null
    }
  }

  scheduleReconnect() {
    if (this.reconnectTimer) return
    this.reconnectAttempts++
    // Backoff progressivo: 2s, 3s, 5s... até no máximo 10s
    const delay = Math.min(2000 * Math.pow(1.3, this.reconnectAttempts - 1), 10000)
    console.log(`[WS] Reconectando em ${(delay / 1000).toFixed(1)}s (tentativa ${this.reconnectAttempts})...`)

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      if (this.shouldReconnect && this.serverUrl && this.username) {
        this.connect(this.serverUrl, this.username, this.isOperator)
      }
    }, delay)
  }

  disconnect() {
    this.shouldReconnect = false
    this.stopPing()
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    if (this.socket) {
      const oldSocket = this.socket
      this.socket = null
      oldSocket.onopen = null
      oldSocket.onmessage = null
      oldSocket.onerror = null
      oldSocket.onclose = null
      try {
        oldSocket.close(1000, 'User Disconnected')
      } catch (e) {
        // ignore
      }
    }
    this.isConnected = false
    this.emit('status', { status: 'disconnected' })
  }

  /**
   * Envia uma mensagem estruturada pelo WebSocket
   */
  send(type, to, payload = {}) {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      console.warn('[WS] Impossível enviar: socket desconectado (tipo:', type, ')')
      return false
    }

    const message = {
      type,
      from: this.username,
      to: to || '',
      payload
    }

    try {
      this.socket.send(JSON.stringify(message))
      if (type !== 'ping') {
        console.log('[WS] Enviado:', type, message)
      }
      return true
    } catch (err) {
      console.error('[WS] Erro ao enviar mensagem:', err)
      return false
    }
  }

  // Ações de alto nível
  requestSupport(hostname, ip, clientName = '') {
    return this.send('request_support', '', { hostname, ip, client_name: clientName })
  }

  cancelSupport() {
    return this.send('cancel_ticket', '')
  }

  acceptTicket(ticketId, clientUser) {
    return this.send('accept_ticket', '', {
      ticket_id: ticketId,
      client_user: clientUser
    })
  }

  closeSession(ticketId, targetUser) {
    return this.send('session_closed', targetUser, {
      ticket_id: ticketId
    })
  }

  sendOffer(to, offer) {
    return this.send('offer', to, offer)
  }

  sendAnswer(to, answer) {
    return this.send('answer', to, answer)
  }

  sendCandidate(to, candidate) {
    return this.send('candidate', to, candidate)
  }

  // Sistema de eventos
  on(event, handler) {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set())
    }
    this.listeners.get(event).add(handler)
  }

  off(event, handler) {
    if (this.listeners.has(event)) {
      this.listeners.get(event).delete(handler)
    }
  }

  removeAllListeners() {
    this.listeners.clear()
  }

  emit(event, data) {
    if (this.listeners.has(event)) {
      this.listeners.get(event).forEach((handler) => {
        try {
          handler(data)
        } catch (err) {
          console.error(`[WS] Erro no handler do evento "${event}":`, err)
        }
      })
    }
  }
}

export const wsService = new WebSocketService()
