# Browser-Based Live Streaming Setup

WebRTC live streaming from a browser (WHIP) and playback in a browser (WHEP) via a cloud media server. This document explains every configuration decision and the reasoning behind it.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│  Browser (Laptop / Mobile)                                          │
│                                                                     │
│  ┌──────────────────┐          ┌──────────────────────────────────┐ │
│  │  Sender (WHIP)   │          │  Viewer (WHEP)                   │ │
│  │  Firefox/Chrome  │          │  Firefox/Chrome                  │ │
│  │  RTCPeerConn.    │          │  RTCPeerConn.                    │ │
│  └────────┬─────────┘          └───────────────┬──────────────────┘ │
│           │ SDP offer (HTTP POST)              │ SDP offer (HTTP POST)│
│           │ WebRTC/SRTP media (UDP)            │ WebRTC/SRTP media (UDP)│
└───────────┼────────────────────────────────────┼─────────────────────┘
            │                                    │
            ▼                                    ▼
┌───────────────────────────────────────────────────────────────────────┐
│  AWS EC2 (eu-central-1)  –  Elastic IP: 18.196.24.10                 │
│  Private IP: 10.0.33.191                                              │
│                                                                       │
│  ┌─────────────────────────────┐   ┌─────────────────────────────┐   │
│  │  MediaMTX  (Docker)         │   │  coturn  (Docker)            │   │
│  │  :8889 HTTP (WHIP/WHEP)     │   │  :3478 UDP/TCP (STUN/TURN)  │   │
│  │  :8189 UDP  (ICE/WebRTC)    │   │  :49152-65535 UDP (relay)   │   │
│  └─────────────────────────────┘   └─────────────────────────────┘   │
└───────────────────────────────────────────────────────────────────────┘
```

**Signal flow (sending):**
1. Browser POSTs an SDP offer to `http://18.196.24.10:8889/live/whip` (WHIP)
2. MediaMTX responds with an SDP answer containing its ICE candidates
3. WebRTC ICE negotiation establishes a direct UDP path
4. Browser streams audio/video via SRTP over the ICE-selected UDP path

**Signal flow (viewing):**
1. Browser POSTs an SDP offer to `http://18.196.24.10:8889/live/whep` (WHEP)
2. MediaMTX responds with the SDP answer for the active stream
3. WebRTC ICE establishes UDP path; MediaMTX forwards the SRTP stream to the viewer

---

## Technology Stack

| Component | Role | Port(s) |
|-----------|------|---------|
| **MediaMTX** | WebRTC media server; accepts WHIP publishes and distributes to WHEP subscribers | TCP 8889 (HTTP), UDP 8189 (ICE) |
| **coturn** | STUN + TURN server; allows browsers behind NAT to establish WebRTC connectivity | UDP/TCP 3478, UDP 49152–65535 (relay) |
| **AWS CDK** | Provisions EC2, Elastic IP, Security Group | — |
| **React/Vite frontend** | Browser UI for sending (WHIP) and viewing (WHEP) | — |

### Key Protocols

- **WHIP** (WebRTC HTTP Ingest Protocol, RFC 9725): Browser publishes a stream by POSTing an SDP offer to a URL. The server returns an SDP answer. The Location header in the 201 response is the URL for session management (DELETE to stop).
- **WHEP** (WebRTC HTTP Egress Protocol, RFC 9736): Browser subscribes by POSTing an SDP offer. Works identically to WHIP on the HTTP level.
- **ICE** (Interactive Connectivity Establishment, RFC 8445): Both sides collect "candidates" (IP:port combinations) and probe all pairs to find a working network path.
- **DTLS** (RFC 9147): After ICE succeeds, a DTLS handshake encrypts the connection and derives SRTP keying material.
- **SRTP**: The actual audio/video stream, encrypted with keys derived from the DTLS handshake.

---

## AWS Infrastructure (CDK)

### Security Group Rules

```typescript
// TCP – management and signalling
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(22));    // SSH
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(8889));  // MediaMTX HTTP (WHIP/WHEP signalling)
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(8554));  // RTSP (optional)
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(9997));  // MediaMTX API

// UDP – ICE media path
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.udp(8189));  // MediaMTX ICE mux

// STUN/TURN
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(3478));
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.udp(3478));
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(5349));  // TURN-TLS
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.udp(5349));

// TURN relay ports
sg.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.udpRange(49152, 65535));
```

**Why UDP 8189 is critical:** MediaMTX uses a single "ICE UDP mux" on port 8189. All WebRTC media traffic between browsers and MediaMTX flows through this one port. If it is blocked in the Security Group, ICE will never establish – the SDP signalling (HTTP) will succeed but no media will flow. This is a common mistake: the HTTP port (8889) and the UDP media port (8189) are both required.

**Why TURN relay ports (49152–65535) are needed:** When a browser is behind a symmetric NAT or restrictive firewall, it allocates a TURN relay port on the coturn server. That relay port is picked from this range. Without this rule, TURN relay candidates cannot be used.

### Elastic IP

The server must have a static Elastic IP (EIP). WebRTC ICE requires a known, stable IP address to function. EC2 public IPs change on every restart without an EIP.

**AWS EIP and hairpin NAT:** AWS translates the EIP externally. The EC2 instance itself only has the private IP (`10.0.33.191`); it never "sees" its own EIP on a network interface. This means:
- The instance cannot reach `18.196.24.10` (its own EIP) directly – AWS blocks hairpin NAT
- coturn and MediaMTX must be configured with both the private IP (for binding) and the public EIP (for advertisement)

---

## coturn Configuration

File: `docker/turnserver.conf`

```conf
listening-port=3478
listening-ip=0.0.0.0

# Relay on the private interface, but advertise the public EIP.
# coturn binds relay sockets to 10.0.33.191 (so it can send and receive).
# The external-ip mapping tells coturn to advertise 18.196.24.10 in its
# XOR-RELAYED-ADDRESS responses so browsers know which IP to use.
relay-ip=10.0.33.191
external-ip=18.196.24.10/10.0.33.191

min-port=49152
max-port=65535

fingerprint
lt-cred-mech
user=streaming:TurnPass2024!
realm=streaming.local
log-file=stdout
verbose
```

**Why `relay-ip=10.0.33.191` and not `0.0.0.0`:**
coturn must bind its relay sockets to an IP it can actually send packets from. The instance only has `10.0.33.191`. Using `0.0.0.0` causes relay sockets to be advertised with `0.0.0.0`, which is unusable by the browser.

**Why `external-ip=18.196.24.10/10.0.33.191`:**
The `external-ip=PUBLIC/PRIVATE` syntax tells coturn: "when relaying on `10.0.33.191`, tell clients the relay address is `18.196.24.10`." The browser then connects to `18.196.24.10:RELAY_PORT`. AWS routes that to `10.0.33.191:RELAY_PORT` where coturn is listening.

---

## MediaMTX Configuration

File: `docker/mediamtx.yml`

```yaml
logLevel: debug   # set to "info" in production
logDestinations: [stdout]

api: yes
apiAddress: :9997

rtsp: yes
rtspAddress: :8554

webrtc: yes
webrtcAddress: :8889       # HTTP signalling (WHIP/WHEP)
webrtcEncryption: no
webrtcAllowOrigins:
  - "*"

# Only advertise the public EIP as a host ICE candidate.
# See "ICE Candidate Explosion" below for why this is necessary.
webrtcIPsFromInterfaces: false
webrtcAdditionalHosts:
  - 18.196.24.10

# No webrtcICEServers2 – see "STUN/TURN on the server side" below.

webrtcHandshakeTimeout: 60s

paths:
  all_others:
```

### Why `webrtcIPsFromInterfaces: false`

By default (`true`), MediaMTX runs Pion's ICE agent which collects host candidates from **all** network interfaces it can see. Because both MediaMTX and coturn use `network_mode: host` in Docker, MediaMTX sees every interface on the EC2 host:

| Interface | IP | Reachable from internet? |
|-----------|-----|--------------------------|
| `lo` | 127.0.0.1 | No |
| `ens5` | 10.0.33.191 | No (private) |
| `docker0` | 172.17.0.1 | No |
| bridge networks | 172.18.0.1, 172.19.0.1, … | No |
| (via `webrtcAdditionalHosts`) | 18.196.24.10 | Yes |

With four host candidates, ICE creates `N_browser_candidates × 4` candidate pairs. Most are unreachable: the browser cannot route packets to `10.0.33.191`, `172.17.0.1`, etc. Each pair times out after several seconds. Firefox checks all high-priority (host×host) pairs before trying lower-priority (srflx×host) ones. With many failing pairs, the 30–60 s handshake timeout expired before ICE found the working pair.

Setting `webrtcIPsFromInterfaces: false` suppresses all interface candidates. Only `18.196.24.10:8189` (from `webrtcAdditionalHosts`) appears in the SDP answer. ICE now succeeds in under a second.

### Why no `webrtcICEServers2`

Adding a STUN or TURN server to `webrtcICEServers2` causes MediaMTX to gather additional ICE candidates for itself. The problem:

- **STUN** produces an `srflx` candidate at `18.196.24.10:RANDOM_PORT`. That random port is **not** in the Security Group (only 8189 is). Browsers trying to reach this candidate time out.
- **TURN** also produces an `srflx` candidate (via the XOR-MAPPED-ADDRESS in the ALLOCATE response) with the same issue.

Without `webrtcICEServers2`, MediaMTX advertises only the `host` candidate on port 8189, which is open and correct.

### Why `webrtcHandshakeTimeout: 60s`

The default is 10 s, which is too short when ICE is also trying relay candidates. 60 s gives enough headroom for the full ICE + DTLS handshake even on slow or congested paths.

---

## Docker Compose

File: `/opt/streaming/docker-compose.yml` (on server)

```yaml
services:
  coturn:
    image: coturn/coturn:4.6
    network_mode: host
    restart: unless-stopped
    volumes:
      - ./turnserver.conf:/etc/coturn/turnserver.conf:ro
    command: -c /etc/coturn/turnserver.conf

  mediamtx:
    image: bluenviron/mediamtx:latest
    network_mode: host
    restart: unless-stopped
    volumes:
      - ./mediamtx.yml:/mediamtx.yml:ro
```

**Why `network_mode: host`:**
Both services need to bind to specific IP addresses (the private EC2 IP) and listen on a range of UDP ports. Bridge networking with port mappings would require mapping hundreds of ports for the TURN relay range. Host networking gives each container direct access to all host interfaces and ports without mapping overhead.

---

## Frontend

### Architecture

The frontend is a React/Vite single-page application with **no external UI library**. All WebRTC logic lives in two custom hooks; the components are thin wrappers that handle layout and user interaction.

```
frontend/
├── src/
│   ├── main.tsx              React entry point (mounts <App>)
│   ├── App.tsx               Root component: tab navigation + Sender + Viewer
│   ├── config.ts             All server addresses, ICE server config
│   ├── hooks/
│   │   ├── useWHIPSender.ts  Publishes camera/screen via WHIP
│   │   └── useWHEPPlayer.ts  Subscribes to a stream via WHEP
│   └── index.css             Minimal global reset
├── package.json              Dependencies: react, react-dom, vite, typescript
└── vite.config.ts            Vite build config
```

**Data flow:**

```
App
 ├── <Sender> (always mounted, hidden via CSS when inactive)
 │    └── useWHIPSender → RTCPeerConnection → WHIP POST → MediaMTX
 └── <Viewer> (always mounted, hidden via CSS when inactive)
      └── useWHEPPlayer → RTCPeerConnection → WHEP POST → MediaMTX
```

Both components stay mounted permanently (see [Keeping the Sender Alive](#keeping-the-sender-alive-while-switching-tabs)) so switching tabs does not close an active stream.

---

### Development and Build

```bash
cd frontend
npm install          # install dependencies (first time only)
npm run dev          # start Vite dev server at http://localhost:5173
```

To build a production bundle:

```bash
npm run build        # outputs to frontend/dist/
npm run preview      # preview the production build locally
```

The `dist/` directory contains a fully static site (HTML + JS + CSS) that can be served by any web server (nginx, Caddy, GitHub Pages, etc.). No Node.js runtime is needed in production.

> **Note on CORS:** the dev server runs on `localhost:5173` while MediaMTX is on `18.196.24.10:8889`. MediaMTX is configured with `webrtcAllowOrigins: ["*"]` to permit this cross-origin fetch. In production, restrict this to the actual frontend domain.

---

### `src/config.ts`

```typescript
export const SERVER_IP   = "18.196.24.10";
export const MEDIAMTX_HTTP = `http://${SERVER_IP}:8889`;
export const MEDIAMTX_API  = `http://${SERVER_IP}:9997`;

export const ICE_SERVERS: RTCIceServer[] = [
  { urls: `stun:${SERVER_IP}:3478` },
  { urls: `turn:${SERVER_IP}:3478`, username: "streaming", credential: "TurnPass2024!" },
  // TURN over TCP: fallback for mobile networks (5G/LTE) that block UDP
  { urls: `turn:${SERVER_IP}:3478?transport=tcp`, username: "streaming", credential: "TurnPass2024!" },
];

export const DEFAULT_STREAM = "live";
```

The browser uses these ICE servers to gather its own candidates:
- **STUN** gives the browser its `srflx` candidate (the public IP:port as seen by the internet).
- **TURN (UDP)** gives the browser a `relay` candidate via UDP. This is the standard fallback when no direct or srflx path works.
- **TURN (TCP)** gives the browser a `relay` candidate via TCP. This is the fallback when the mobile carrier blocks UDP (common on 5G/LTE). coturn accepts both UDP and TCP on port 3478 by default.

### Why 5G / LTE Requires TCP TURN

Mobile carriers (5G, LTE) commonly use **Carrier-Grade NAT (CGNAT)** and may block outbound UDP to high-numbered ports or even to port 3478. The result:

1. UDP STUN fails → no srflx candidate
2. UDP TURN fails → no relay candidate via UDP
3. Browser sends offer with only private host candidates (`192.0.0.x`, `10.x.x.x`)
4. MediaMTX cannot reach these private CGNAT addresses → ICE fails

TCP TURN works even when UDP is blocked because it tunnels all communication over a TCP connection to port 3478. The relay path then becomes:

```
Browser (5G) ──TCP──► coturn :3478 ──UDP──► MediaMTX :8189
```

coturn handles the protocol translation internally. MediaMTX sees only standard UDP on its ICE mux port.

**The sender and viewer can be on completely different networks.** Each connects independently to MediaMTX on EC2:
- Sender on 5G → WHIP → MediaMTX (via TURN relay if needed)
- Viewer on DSL → WHEP → MediaMTX (typically direct or srflx path)
- MediaMTX distributes the stream from sender to all viewers

### The DTLS Role Fix (`a=setup:active`)

WebRTC DTLS has two roles: **client** (initiates the handshake, sends ClientHello) and **server** (responds with ServerHello). The `a=setup` SDP attribute controls which side takes which role:

| SDP value | DTLS role |
|-----------|-----------|
| `active` | client (sends ClientHello) |
| `passive` | server (responds) |
| `actpass` | either (the other side decides) |

Browser WebRTC implementations default to `a=setup:actpass` in offers. MediaMTX answers with `a=setup:active`, making **MediaMTX the DTLS client** and the browser the DTLS server.

**The bug:** MediaMTX uses the Pion WebRTC library. In the tested version (MediaMTX 1.19.0 / Pion), the DTLS *client* path has a defect: after sending ClientHello, Pion does not correctly process the browser's ServerHello response. It retransmits ClientHello indefinitely (every 1 s, 2 s, 4 s, 8 s …) until the handshake times out.

**The fix:** Force the browser to be the DTLS *client* by modifying the SDP offer before sending it:

```typescript
const offer = await pc.createOffer();
// Change actpass → active so MediaMTX must answer passive (DTLS server).
// Pion's DTLS server path is stable; only the client path has this bug.
const sdp = offer.sdp!.replace(/a=setup:actpass/g, 'a=setup:active');
await pc.setLocalDescription({ type: 'offer', sdp });
```

This must be done in **both** the WHIP sender hook and the WHEP viewer hook.

### Waiting for ICE Gathering Before Sending the Offer

WHIP uses a single HTTP round-trip for offer/answer. Unlike the original JSEP signalling (which supports trickle ICE via separate HTTP PATCH requests), many WHIP server implementations expect all candidates to be present in the initial offer.

The browser's ICE agent continues gathering candidates asynchronously after `setLocalDescription()`. If the SDP is sent immediately, it may include only a partial list of candidates (e.g. only host candidates, no srflx or relay). The server's ICE agent would then try to reach unreachable host addresses and fail.

**The fix:** wait until `iceGatheringState === 'complete'` before POSTing the offer:

```typescript
await new Promise<void>((resolve) => {
  if (pc.iceGatheringState === 'complete') { resolve(); return; }
  const tid = setTimeout(resolve, 5000);   // 5 s safety timeout
  pc.onicegatheringstatechange = () => {
    if (pc.iceGatheringState === 'complete') { clearTimeout(tid); resolve(); }
  };
});
// Now send pc.localDescription!.sdp – it contains all candidates
```

### Location Header Fix

The WHIP 201 response includes a `Location` header with the session URL used for the DELETE request (to cleanly stop the stream). MediaMTX returns a **relative path** (e.g. `/live/whip/UUID`), not an absolute URL.

`fetch()` resolves relative URLs against the current page URL (e.g. `http://localhost:5173`), not the WHIP server URL. This caused DELETE requests to hit the Vite dev server instead of MediaMTX.

**The fix:**

```typescript
const location = res.headers.get('Location');
if (location) {
  locationRef.current = location.startsWith('http')
    ? location
    : new URL(location, new URL(whipUrl).origin).href;
}
```

### Keeping the Sender Alive While Switching Tabs

The React app has two tabs: "Senden" (send) and "Schauen" (watch). A naive conditional render (`{mode === 'send' ? <Sender /> : <Viewer />}`) unmounts the inactive component. When the `Sender` component unmounts, its `useEffect` cleanup runs `pc.close()`, terminating the active WHIP session.

**The fix:** render both components always; hide the inactive one with CSS:

```tsx
<div style={{ display: mode === 'send' ? undefined : 'none' }}><Sender /></div>
<div style={{ display: mode === 'watch' ? undefined : 'none' }}><Viewer /></div>
```

The stream continues in the background while the viewer tab is open.

---

### `App.tsx`: Component Structure

`App` manages a single `mode` state (`'watch' | 'send'`) and renders both `<Sender>` and `<Viewer>` at all times, showing only the active one via CSS.

**`<Sender>`** — renders the WHIP publishing UI:
- Source selector: Webcam + Mikrofon (`getUserMedia`) or Bildschirm (`getDisplayMedia`)
- Stream name input (default `live`) — sent as the path segment in the WHIP URL
- Status badge with animated pulse when live
- ICE candidate inspector button (appears after gathering): shows host / srflx / relay counts and details, colour-coded by type (green = relay, amber = srflx, grey = host)
- Live preview `<video>` fed directly from the captured `MediaStream`

**`<Viewer>`** — renders the WHEP subscriber UI:
- Stream name input
- Autoplay checkbox: if checked, `connect()` is called immediately on mount (useful for embedding)
- Status badge
- Playback `<video>` element: shown opaque only when `status === 'connected'`; an overlay placeholder is shown otherwise

**Status values and transitions:**

```
Sender (SenderStatus):
  idle → gathering → connecting → live
              └──────────────────────→ error
              └──── stopped (stop() called at any point)

Viewer (PlayerStatus):
  idle → connecting → connected
                └────────────→ error
                └────────────→ disconnected
```

| Sender status | Meaning |
|---------------|---------|
| `idle` | Initial state, nothing started |
| `gathering` | `getUserMedia`/`getDisplayMedia` granted; ICE candidates being collected (4–5 s on 5G) |
| `connecting` | SDP offer sent to MediaMTX; waiting for ICE + DTLS to establish |
| `live` | `connectionState === 'connected'`; media is flowing |
| `error` | ICE failed or WHIP returned an error; message shown to user |
| `stopped` | `stop()` was called; DELETE sent to MediaMTX to release the session |

| Viewer status | Meaning |
|---------------|---------|
| `idle` | Not connected; shown when MediaMTX returns 404 (no active stream) |
| `connecting` | SDP offer sent; waiting for ICE + DTLS |
| `connected` | `iceConnectionState === 'connected'`; video playing |
| `error` | ICE failed or unexpected WHEP error |
| `disconnected` | `disconnect()` called or peer connection closed |

---

### `useWHIPSender`: Publishing a Stream

**Signature:**
```typescript
function useWHIPSender(previewRef: React.RefObject<HTMLVideoElement | null>): {
  status:     SenderStatus;
  error:      string | null;
  candidates: RTCIceCandidate[];   // all gathered local ICE candidates
  start:      (whipUrl: string, sourceType: 'camera' | 'screen') => Promise<void>;
  stop:       () => void;
}
```

**`start(whipUrl, sourceType)`** — full sequence:
1. Calls `getUserMedia` (camera) or `getDisplayMedia` (screen); feeds stream into preview `<video>`
2. Creates `RTCPeerConnection` with `ICE_SERVERS`
3. Creates an SDP offer; patches `a=setup:actpass → a=setup:active` (DTLS fix)
4. Calls `setLocalDescription`; waits up to 5 s for `iceGatheringState === 'complete'`
5. POSTs the SDP to `whipUrl` with `Content-Type: application/sdp`
6. Reads the `Location` header; resolves it against the WHIP server origin (not the page origin)
7. Calls `setRemoteDescription` with the SDP answer
8. Listens on `onconnectionstatechange`: sets `status` to `live` / `error` / `stopped`

**`stop()`** — cleanup sequence:
1. DELETEs the session URL (stored in `locationRef`) → tells MediaMTX to release resources
2. Closes `RTCPeerConnection`
3. Stops all `MediaStreamTrack`s (releases camera / mic / screen capture)
4. Clears the preview `<video>` `srcObject`

The hook registers `useEffect(() => () => stop(), [])` so the stream is always cleaned up on unmount — but because of the `display: none` pattern, unmount only happens when the user closes or navigates away from the page.

---

### `useWHEPPlayer`: Subscribing to a Stream

**Signature:**
```typescript
function useWHEPPlayer(videoRef: React.RefObject<HTMLVideoElement | null>): {
  status:     PlayerStatus;
  error:      string | null;
  connect:    (whepUrl: string) => Promise<void>;
  disconnect: () => void;
}
```

**`connect(whepUrl)`** — sequence:
1. Creates `RTCPeerConnection` with `ICE_SERVERS`
2. Adds two `recvonly` transceivers (video + audio)
3. Creates SDP offer; patches `a=setup:actpass → a=setup:active` (same DTLS fix)
4. Waits up to 4 s for `iceGatheringState === 'complete'`
5. POSTs the SDP to `whepUrl`
6. If MediaMTX returns 404, sets `status = 'idle'` and `error = 'Kein Stream aktiv'` — no retry loop, user must click again
7. Sets the SDP answer as remote description
8. `ontrack` event feeds the incoming `MediaStream` into the playback `<video>`
9. Listens on `oniceconnectionstatechange` for `connected` / `failed` / `disconnected`

**`disconnect()`** — closes the peer connection and clears the video element.

---

### Integrating the Hooks in Another Application

The two hooks are self-contained and have no dependency on `App.tsx`. To embed them:

```typescript
import { useRef } from 'react';
import { useWHIPSender } from './hooks/useWHIPSender';
import { useWHEPPlayer } from './hooks/useWHEPPlayer';
import { MEDIAMTX_HTTP } from './config';

// In your component:
const videoRef = useRef<HTMLVideoElement>(null);
const { status, start, stop } = useWHIPSender(videoRef);

// Start publishing from webcam:
start(`${MEDIAMTX_HTTP}/mystream/whip`, 'camera');

// Or subscribe:
const playerRef = useRef<HTMLVideoElement>(null);
const { connect, disconnect } = useWHEPPlayer(playerRef);
connect(`${MEDIAMTX_HTTP}/mystream/whep`);
```

The only runtime dependencies are `react` and `react-dom`. No additional WebRTC library is needed — the hooks use the browser's native `RTCPeerConnection` API directly.

---

## ICE Candidate Types and Priority

ICE tries candidate pairs in priority order (highest first):

| Type | Example | Description |
|------|---------|-------------|
| `host` | `192.168.1.10:PORT` | Device's own LAN IP |
| `srflx` (server-reflexive) | `88.150.107.148:PORT` | Public IP, discovered via STUN |
| `relay` | `18.196.24.10:PORT` | TURN relay – always works, highest latency |

For a pair to succeed, both sides must be able to reach each other. A browser behind a home router (NAT) cannot be reached directly from the server, but the server can reach the browser's **srflx** address because the NAT table was opened by the browser's outbound STUN check.

In this setup, the working path is typically:
```
Browser srflx (88.x.x.x:PORT) ←→ MediaMTX host (18.196.24.10:8189)
```

If the home router has a restrictive NAT (symmetric NAT), the TURN relay path is used instead:
```
Browser relay (18.196.24.10:RELAY) ←→ MediaMTX host (18.196.24.10:8189)
                  ↕ (coturn forwards both directions)
```

---

## Debugging Tools

### Check if a UDP port is reachable

```python
# Run from the client machine
import socket
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.settimeout(3)
msg = b'\x00\x01\x00\x00' + b'\x21\x12\xA4\x42' + b'\x00' * 12  # STUN binding request
s.sendto(msg, ('18.196.24.10', 3478))  # test STUN on coturn
print(s.recvfrom(1024))  # should return a response
```

A response means the port is open. A timeout means the Security Group is blocking it.

### Capture ICE traffic on the server

```bash
# On EC2 – captures all UDP traffic on port 8189 for 30 seconds
sudo tcpdump -i any -n udp port 8189 -c 500 -w /tmp/ice.pcap
# Read and display
sudo tcpdump -r /tmp/ice.pcap -n | head -50
```

Look for:
- **Out** packets to the browser's public IP → MediaMTX is sending STUN checks ✓
- **In** packets from the browser's public IP → browser's STUN checks are arriving ✓
- No **In** packets → Security Group is blocking UDP 8189 ✗

### MediaMTX debug logging

Set `logLevel: debug` in `mediamtx.yml` to see:
- SDP offer and answer (with all ICE candidates)
- Session creation and closure
- Peer connection state changes (connecting → connected / closed)

### Firefox `about:webrtc`

Open `about:webrtc` in Firefox while a WebRTC session is active. It shows:
- All gathered ICE candidates (host, srflx, relay)
- ICE pair check results (succeeded / failed)
- DTLS state and fingerprints
- Active media tracks

---

## Common Mistakes and Solutions

| Problem | Symptom | Cause | Fix |
|---------|---------|-------|-----|
| UDP 8189 not open | ICE never establishes; no inbound packets in tcpdump | CDK deployed without the UDP 8189 rule, or rule added after last deploy | Run `cdk deploy` to apply the Security Group update |
| Too many MediaMTX ICE candidates | ICE takes 30–60 s and times out | `webrtcIPsFromInterfaces: true` → MediaMTX advertises all host interfaces | Set `webrtcIPsFromInterfaces: false` |
| DTLS handshake loops forever | Stream never reaches "Live"; MediaMTX logs show session closed after 60 s | Pion DTLS client bug: ServerHello not processed | Change `a=setup:actpass` to `a=setup:active` in the SDP offer |
| MediaMTX srflx on blocked ports | ICE fails after 10 s timeout | `webrtcICEServers2` causes MediaMTX to gather srflx on ephemeral ports not in the Security Group | Remove `webrtcICEServers2` from `mediamtx.yml` |
| DELETE hits localhost:5173 | Console: `404 DELETE http://localhost:5173/live/whip/UUID` | `Location` header is relative; resolved against dev server URL | Resolve against WHIP server origin: `new URL(location, new URL(whipUrl).origin).href` |
| Stream stops on tab switch | WHEP says "no stream available" | React unmounts `<Sender>` on tab change, triggering `pc.close()` | Use `display: none` instead of conditional rendering |
| TURN "broken" warning in Firefox | Console warning on page load | WHEP auto-connect on load, 404 response, ICE fails mid-gathering | Handle 404 as "idle" state; do not auto-play on load |
| Browser on 5G/LTE: ICE fails, no relay candidates | Browser offer has only private CGNAT IPs (192.0.0.x, 10.x.x.x), no srflx or relay | 5G carrier blocks UDP to port 3478 → STUN and UDP-TURN fail → no public candidates | Add `turn:SERVER:3478?transport=tcp` to ICE_SERVERS in config.ts |

---

## Deployment Checklist

1. **CDK deploy** – run `npx cdk deploy` from the `cdk/` directory whenever `streaming-stack.ts` changes. The Security Group rules only take effect after a deploy.
2. **Copy config files to server:**
   ```bash
   scp docker/mediamtx.yml   ec2-user@SERVER:/tmp/
   scp docker/turnserver.conf ec2-user@SERVER:/tmp/
   ssh ec2-user@SERVER "sudo cp /tmp/mediamtx.yml /opt/streaming/ && sudo cp /tmp/turnserver.conf /opt/streaming/"
   ```
3. **Restart services:**
   ```bash
   ssh ec2-user@SERVER "cd /opt/streaming && sudo docker compose restart"
   ```
4. **Verify:**
   - STUN reachable: Python STUN test on port 3478 → response received
   - ICE port open: Python STUN test on port 8189 → no response is OK (Pion drops unauthenticated STUN); check via `tcpdump` during a session
   - Test stream: open the frontend, start sending, open a second browser tab and watch
