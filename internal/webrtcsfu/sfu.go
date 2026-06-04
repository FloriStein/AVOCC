// Package webrtcsfu implements the WebRTC Selective Forwarding Unit (BE-08, ADR-014/015).
// The SFU is a Dumb Media Router with State Subscription — it consumes session events
// from the Control Server but never influences system state (ADR-007 Invariant).
package webrtcsfu

import (
	"log"
	"sync"

	"github.com/pion/webrtc/v4"
)

// SessionEventType mirrors the session event types from the Control Server (ADR-015).
type SessionEventType string

const (
	EventCreated          SessionEventType = "SESSION_CREATED"
	EventOperatorAssigned SessionEventType = "SESSION_OPERATOR_ASSIGNED"
	EventOperatorHandover SessionEventType = "SESSION_OPERATOR_HANDOVER"
	EventDegraded         SessionEventType = "SESSION_DEGRADED"
	EventSafeMode         SessionEventType = "SESSION_SAFE_MODE"
	EventEnded            SessionEventType = "SESSION_ENDED"
)

// SessionEvent carries context pushed from the Control Server (ADR-015).
type SessionEvent struct {
	Type       SessionEventType `json:"type"`
	SessionID  string           `json:"session_id"`
	OperatorID string           `json:"operator_id"`
}

// Peer represents one WebRTC peer connection (vehicle or operator).
type Peer struct {
	ID         string
	Role       string // "vehicle" or "operator"
	SessionID  string
	Connection *webrtc.PeerConnection
	Tracks     []*webrtc.TrackLocalStaticRTP
}

// SFU manages peer connections and routes media tracks.
type SFU struct {
	mu      sync.RWMutex
	api     *webrtc.API
	peers   map[string]*Peer    // keyed by peerID
	routing map[string][]string // sessionID → operator peer IDs
	state   map[string]SessionEventType // sessionID → last event type
}

func New() *SFU {
	// MediaEngine: enable VP8 + Opus (browser defaults)
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		log.Fatalf("[SFU] codec registration failed: %v", err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))
	return &SFU{
		api:     api,
		peers:   make(map[string]*Peer),
		routing: make(map[string][]string),
		state:   make(map[string]SessionEventType),
	}
}

// HandleSessionEvent processes a session lifecycle event from the Control Server.
// The SFU consumes but never interprets safety-critical state (ADR-007/015).
func (s *SFU) HandleSessionEvent(event SessionEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state[event.SessionID] = event.Type
	log.Printf("[SFU] session event: %s session=%s operator=%s",
		event.Type, event.SessionID, event.OperatorID)

	switch event.Type {
	case EventCreated:
		s.routing[event.SessionID] = []string{}

	case EventOperatorAssigned, EventOperatorHandover:
		// Track which operator(s) should receive the primary stream.
		s.routing[event.SessionID] = []string{event.OperatorID}

	case EventSafeMode:
		// Drop all active streams immediately (ADR-015).
		s.dropStreams(event.SessionID)

	case EventEnded:
		s.dropStreams(event.SessionID)
		delete(s.routing, event.SessionID)
		delete(s.state, event.SessionID)
	}
}

// CreateVehicleOffer accepts a WebRTC offer from a vehicle and returns an SDP answer.
// The vehicle's tracks are stored for forwarding to subscribed operators.
func (s *SFU) CreateVehicleOffer(sessionID, peerID, sdpOffer string) (string, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun-turn:3478"}},
		},
	}

	pc, err := s.api.NewPeerConnection(config)
	if err != nil {
		return "", err
	}

	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		log.Printf("[SFU] vehicle track: session=%s kind=%s codec=%s",
			sessionID, track.Kind(), track.Codec().MimeType)
		go s.forwardTrack(sessionID, track)
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("[SFU] vehicle ICE: %s (session=%s)", state, sessionID)
		if state == webrtc.ICEConnectionStateFailed || state == webrtc.ICEConnectionStateDisconnected {
			s.removePeer(peerID)
		}
	})

	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sdpOffer}
	if err := pc.SetRemoteDescription(offer); err != nil {
		pc.Close()
		return "", err
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		return "", err
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		pc.Close()
		return "", err
	}

	// Wait for ICE gathering to complete (simplified — production uses Trickle ICE)
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	<-gatherComplete

	s.mu.Lock()
	s.peers[peerID] = &Peer{
		ID:         peerID,
		Role:       "vehicle",
		SessionID:  sessionID,
		Connection: pc,
	}
	s.mu.Unlock()

	log.Printf("[SFU] vehicle connected: peer=%s session=%s", peerID, sessionID)
	return pc.LocalDescription().SDP, nil
}

// forwardTrack routes an incoming vehicle RTP track to all subscribed operator connections.
func (s *SFU) forwardTrack(sessionID string, track *webrtc.TrackRemote) {
	rtpBuf := make([]byte, 1400)
	for {
		n, _, err := track.Read(rtpBuf)
		if err != nil {
			return
		}

		s.mu.RLock()
		operatorIDs := s.routing[sessionID]
		safeModeActive := s.state[sessionID] == EventSafeMode
		s.mu.RUnlock()

		if safeModeActive {
			continue // SESSION_SAFE_MODE → no forwarding (ADR-015)
		}

		for _, opID := range operatorIDs {
			s.mu.RLock()
			peer, ok := s.peers[opID]
			s.mu.RUnlock()
			if !ok {
				continue
			}
			for _, localTrack := range peer.Tracks {
				if _, err := localTrack.Write(rtpBuf[:n]); err != nil {
					log.Printf("[SFU] forward error to operator %s: %v", opID, err)
				}
			}
		}
	}
}

func (s *SFU) dropStreams(sessionID string) {
	for id, peer := range s.peers {
		if peer.SessionID == sessionID {
			peer.Connection.Close()
			delete(s.peers, id)
			log.Printf("[SFU] dropped peer: %s (session=%s)", id, sessionID)
		}
	}
}

// SubscribeOperator accepts a WebRTC offer from an operator browser and returns an SDP answer.
// The operator's PeerConnection receives tracks forwarded from the vehicle (ADR-014).
func (s *SFU) SubscribeOperator(sessionID, operatorID, sdpOffer string) (string, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun-turn:3478"}},
		},
	}

	pc, err := s.api.NewPeerConnection(config)
	if err != nil {
		return "", err
	}

	// Create a local track so the SFU can forward vehicle RTP to this operator (ADR-014).
	localTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
		"video", "avoc-vehicle",
	)
	if err != nil {
		pc.Close()
		return "", err
	}
	if _, err := pc.AddTrack(localTrack); err != nil {
		pc.Close()
		return "", err
	}

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("[SFU] operator ICE: %s (session=%s operator=%s)", state, sessionID, operatorID)
		if state == webrtc.ICEConnectionStateFailed || state == webrtc.ICEConnectionStateDisconnected {
			s.removePeer(operatorID)
		}
	})

	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sdpOffer}
	if err := pc.SetRemoteDescription(offer); err != nil {
		pc.Close()
		return "", err
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		return "", err
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		pc.Close()
		return "", err
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	<-gatherComplete

	s.mu.Lock()
	s.peers[operatorID] = &Peer{
		ID:         operatorID,
		Role:       "operator",
		SessionID:  sessionID,
		Connection: pc,
		Tracks:     []*webrtc.TrackLocalStaticRTP{localTrack},
	}
	// Register this operator in the session routing table.
	existing := s.routing[sessionID]
	for _, id := range existing {
		if id == operatorID {
			s.mu.Unlock()
			log.Printf("[SFU] operator already subscribed: %s", operatorID)
			return pc.LocalDescription().SDP, nil
		}
	}
	s.routing[sessionID] = append(existing, operatorID)
	s.mu.Unlock()

	log.Printf("[SFU] operator subscribed: peer=%s session=%s", operatorID, sessionID)
	return pc.LocalDescription().SDP, nil
}

func (s *SFU) removePeer(peerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if peer, ok := s.peers[peerID]; ok {
		peer.Connection.Close()
		delete(s.peers, peerID)
	}
}
