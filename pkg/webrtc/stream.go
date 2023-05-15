package webrtc

import (
	"encoding/json"
	"log"
	"os"
	"sync"

	"github.com/gofiber/websocket/v2"
	"github.com/pion/webrtc/v3"
)

func StreamConn(c *websocket.Conn, p *Peers) {
	config := webrtc.Configuration{}

	if os.Getenv("ENVIRONMENT") == "PRODUCTION" {
		config = turnConfig
	}

	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Println("error in creating new peer connection", err.Error())
		return
	}
	defer peerConnection.Close()

	for _, typ := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
		if _, err := peerConnection.AddTransceiverFromKind(typ, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionRecvonly,
		}); err != nil {
			log.Println("error in adding transreceiver", err.Error())
			return
		}
	}

	newPeer := PeerConnectionState{
		PeerConnection: peerConnection,
		Websocket: &ThreadSafeWriter{
			Conn:  c,
			Mutex: sync.Mutex{},
		},
	}

	p.ListLock.Lock()
	p.Connections = append(p.Connections, newPeer)
	p.ListLock.Unlock()

	log.Println("total connections ", p.Connections)

	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
			return
		}
		candidateString, err := json.Marshal(i.ToJSON())
		if err != nil {
			log.Println("error in marshalling candidate data for stream", err.Error())
			return
		}
		if writeErr := newPeer.Websocket.WriteJSON(&websocketMessage{
			Event: "candidate",
			Data:  string(candidateString),
		}); writeErr != nil {
			log.Println("wrirte error in candidate data", writeErr.Error())
		}
	})

	peerConnection.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		switch pcs {
		case webrtc.PeerConnectionStateFailed:
			if err := peerConnection.Close(); err != nil {
				log.Println("err in closing peer connection", err.Error())
			}
		case webrtc.PeerConnectionStateClosed:
			p.SignalPeerConnections()
		}
	})

	p.SignalPeerConnections()
	message := websocketMessage{}
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			log.Println("error in reading message", err.Error())
			return
		} else if err := json.Unmarshal(raw, &message); err != nil {
			log.Println("error in unmarshal of read data", err.Error())
			return
		}

		switch message.Event {
		case "candidate":
			candidate := webrtc.ICECandidateInit{}
			if err := json.Unmarshal([]byte(message.Data), &candidate); err != nil {
				log.Println("error in unmarshalling candidate event data", err.Error())
				return
			}
			if err := peerConnection.AddICECandidate(candidate); err != nil {
				log.Println("error in adding ice candidate", err.Error())
				return
			}

		case "answer":
			answer := webrtc.SessionDescription{}
			if err := json.Unmarshal([]byte(message.Data), &answer); err != nil {
				log.Println("error in unmarshalling session data", err.Error())
				return
			}
			if err := peerConnection.SetRemoteDescription(answer); err != nil {
				log.Println("error in setting answer", err.Error())
				return
			}
		}
	}
}
