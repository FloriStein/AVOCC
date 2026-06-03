// WebSocket client for the AVOC Control Channel (ADR-010/012b).
// Sends Protobuf-encoded ControlCommand messages, receives {"ack":true} JSON responses.
// Latency = round-trip from send to ACK receipt.

export type AckHandler = (latencyMs: number) => void
export type CloseHandler = () => void

export class WSClient {
  private ws: WebSocket | null = null
  private pendingAckTs = 0

  onClose: CloseHandler | null = null
  onAck: AckHandler | null = null

  connect(token: string): void {
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const url = `${proto}://${window.location.host}/ws?token=${token}`
    this.ws = new WebSocket(url)
    this.ws.binaryType = 'arraybuffer'

    this.ws.onmessage = (e) => {
      if (this.pendingAckTs > 0) {
        const latency = Date.now() - this.pendingAckTs
        this.pendingAckTs = 0
        this.onAck?.(latency)
      }
      // Future: parse Protobuf ControlAck (BE-04)
      void e
    }

    this.ws.onclose = () => {
      this.ws = null
      this.onClose?.()
    }
  }

  // Send binary Protobuf-encoded command bytes.
  // Records send timestamp for ACK latency measurement.
  send(bytes: Uint8Array): void {
    if (this.ws?.readyState !== WebSocket.OPEN) return
    this.pendingAckTs = Date.now()
    this.ws.send(bytes)
  }

  disconnect(): void {
    this.ws?.close()
    this.ws = null
  }

  isOpen(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }
}
