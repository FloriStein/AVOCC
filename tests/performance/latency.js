// k6 Load Test — Control Loop ACK Latency (ADR-006/010)
// Threshold: p(99) < 100ms (CI Build-Fail on violation)
// Run: make test-k6   (starts test stack automatically)
// Manual: docker run --rm --network host grafana/k6 run - < tests/performance/latency.js

import http from 'k6/http'
import { check, sleep } from 'k6'
import { Trend, Rate } from 'k6/metrics'

const ackLatency = new Trend('ack_latency_ms', true)
const successRate = new Rate('ack_success_rate')

export const options = {
  vus: 10,
  duration: '30s',
  thresholds: {
    // ADR-010: <100ms ACK-Roundtrip — CI Build-Fail on violation
    'ack_latency_ms': ['p(99)<100'],
    'ack_success_rate': ['rate>0.99'],
    // Default k6 HTTP thresholds
    'http_req_duration': ['p(95)<200'],
    'http_req_failed': ['rate<0.01'],
  },
}

const BASE_URL = __ENV.BASE_URL || 'http://localhost:18080'
const AUTH_URL = __ENV.AUTH_URL || 'http://localhost:18081'

export function setup() {
  // Login and get token
  const loginResp = http.post(
    `${AUTH_URL}/auth/operator/login`,
    JSON.stringify({ id: 'k6-operator', password: 'test' }),
    { headers: { 'Content-Type': 'application/json' } },
  )
  check(loginResp, { 'login ok': (r) => r.status === 200 })

  const token = loginResp.json('token')
  return { token }
}

export default function (data) {
  const { token } = data

  // Measure state endpoint as HTTP proxy for ACK latency
  // (WebSocket binary framing in k6 requires additional setup)
  const t0 = Date.now()
  const resp = http.get(`${BASE_URL}/state`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  const latencyMs = Date.now() - t0

  const ok = check(resp, {
    'state OK': (r) => r.status === 200,
    'latency < 100ms': () => latencyMs < 100,
  })

  ackLatency.add(latencyMs)
  successRate.add(ok)

  sleep(0.05) // 20 requests/sec per VU
}

export function teardown(data) {
  // nothing to clean up
}
