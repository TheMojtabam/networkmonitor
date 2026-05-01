// Mirrors backend/internal/collector/types.go.
// Keep these in sync — the JSON shape is the contract.

export type Protocol = 'tcp' | 'tcp6' | 'udp' | 'udp6'

export type PortStateName =
  | 'ESTABLISHED'
  | 'SYN_SENT'
  | 'SYN_RECV'
  | 'FIN_WAIT1'
  | 'FIN_WAIT2'
  | 'TIME_WAIT'
  | 'CLOSE'
  | 'CLOSE_WAIT'
  | 'LAST_ACK'
  | 'LISTEN'
  | 'CLOSING'
  | 'UNKNOWN'

export interface InterfaceStats {
  name: string
  rxBytes: number
  rxPackets: number
  rxErrors: number
  rxDropped: number
  txBytes: number
  txPackets: number
  txErrors: number
  txDropped: number
  ts: string
}

export interface InterfaceRate {
  name: string
  rxBytesPerSec: number
  txBytesPerSec: number
  rxPktsPerSec: number
  txPktsPerSec: number
  ts: string
}

export interface Port {
  protocol: Protocol
  localAddr: string
  localIp: string
  localPort: number
  state: number
  stateName: PortStateName
  process?: string
  pid?: number
  connectionCount: number
  rxBytesPerSec: number
  txBytesPerSec: number
  totalBps: number
  ts: string
}

export interface Connection {
  protocol: Protocol
  localAddr: string
  remoteAddr: string
  localIp: string
  localPort: number
  remoteIp: string
  remotePort: number
  state: number
  stateName: PortStateName
  process?: string
  pid?: number
  country?: string
  asn?: string
  rxBytes?: number
  txBytes?: number
  age?: number
  ts: string
}

export interface SnapshotTotals {
  rxBytesPerSec: number
  txBytesPerSec: number
  listeningPorts: number
  activeConns: number
  establishedConn: number
}

export interface Snapshot {
  ts: string
  interfaces: InterfaceStats[]
  rates: InterfaceRate[]
  ports: Port[]
  topPorts: Port[]
  totals: SnapshotTotals
}

export interface HistoryPoint {
  ts: string
  rxBytesPerSec: number
  txBytesPerSec: number
}

// ----- alerts -----

export type AlertOp = '>' | '>=' | '<' | '<=' | '=='

export type AlertMetric =
  | 'port_bps'
  | 'total_rx_bps'
  | 'total_tx_bps'
  | 'total_bps'
  | 'connection_count'

export interface AlertRule {
  id: string
  name: string
  enabled: boolean
  metric: AlertMetric
  operator: AlertOp
  threshold: number
  port?: number
  protocol?: Protocol
  forSeconds: number
  channels: string[]
}

export interface AlertChannel {
  name: string
  type: 'webhook' | 'telegram' | 'email'
  webhook?: string
  telegram?: { chatId?: string }
  email?: {
    smtpHost?: string
    smtpPort?: number
    from?: string
    to?: string
  }
}

export interface AlertEvent {
  ruleId: string
  ruleName: string
  metric: AlertMetric
  value: number
  threshold: number
  port?: number
  ts: string
}

// ----- WebSocket envelope -----

export type WSMessage =
  | { type: 'snapshot'; data: Snapshot }
  | { type: 'alert'; data: AlertEvent }

// ----- auth -----

export interface AuthMe {
  user?: { username: string; role: string }
  authDisabled?: boolean
}
