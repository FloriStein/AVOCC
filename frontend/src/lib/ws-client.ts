// WebSocket client for the AVOC Control Channel (ADR-010/012b).
// Sends Protobuf-encoded ControlCommand messages, receives Protobuf ControlAck responses (BE-04).
// Latency = round-trip from send to ACK receipt.

import { fromBinary } from '@bufbuild/protobuf'

// eslint-disable-next-line @typescript-eslint/no-explicit-any
let ControlAckSchema: any
async function loadAckSchema() {
  try {
    const ctrl = await import('@/gen/control_pb.js')
    ControlAckSchema = ctrl.ControlAckSchema
  } catch {
    // gen/ not yet generated — works after Docker build
  }
}
loadAckSchema()

export type AckHandler = (latencyMs: number) => void
export type CloseHandler = () => void
export type AckErrorHandler = (msg: string) => void

export class WSClient {
  private ws: WebSocket | null = null
  private pendingAckTs = 0

  onClose: CloseHandler | null = null
  onAck: AckHandler | null = null
  onAckError: AckErrorHandler | null = null

  connect(token: string): void {
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const url = `${proto}://${window.location.host}/ws?token=${token}`
    this.ws = new WebSocket(url)
    this.ws.binaryType = 'arraybuffer'

    this.ws.onmessage = (e) => {
      const latency = this.pendingAckTs > 0 ? Date.now() - this.pendingAckTs : 0
      this.pendingAckTs = 0

      if (latency > 0) this.onAck?.(latency)

      // Parse Protobuf ControlAck to surface error_msg from server (BE-04)
      if (ControlAckSchema && e.data instanceof ArrayBuffer && e.data.byteLength > 0) {
        try {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          const ack = fromBinary(ControlAckSchema, new Uint8Array(e.data)) as any
          if (!ack.success && ack.errorMsg) this.onAckError?.(ack.errorMsg as string)
        } catch {
          // non-Protobuf response (e.g. server fallback) — ignore
        }
      }
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
    if (this.ws) {
      this.ws.onclose = null   // prevent onClose callback on intentional close
      this.ws.close()
      this.ws = null
    }
  }

  isOpen(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }
}
