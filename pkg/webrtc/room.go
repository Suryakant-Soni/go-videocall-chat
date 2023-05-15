package webrtc

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gofiber/websocket/v2"
	"github.com/pion/webrtc/v3"
)

func RoomConn(c *websocket.Conn, p *Peers) {
	log.Println("RoomConn func trigerred")
	// webrtc.SetLogLevel(webrtc.LogLevelDebug)
	var config webrtc.Configuration
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Println(err)
		return
	}
	defer peerConnection.Close()

	for _, typ := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
		if _, err := peerConnection.AddTransceiverFromKind(typ, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionRecvonly,
		}); err != nil {
			log.Println("error in adding transreceiver")
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

	// add our peer connection to the global list of peers
	p.ListLock.Lock()
	p.Connections = append(p.Connections, newPeer)
	p.ListLock.Unlock()

	log.Println("my peer Connections", p.Connections)
	log.Println("connection State", p.Connections[0].PeerConnection.ConnectionState())
	//handler for when ice candidate is received
	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		log.Println("OnICECandidate func triggered")
		if i == nil {
			return
		}

		candidateString, err := json.Marshal(i.ToJSON())
		if err != nil {
			log.Println("error in marshalling candidate string", err.Error())
			return
		}

		if writeErr := newPeer.Websocket.WriteJSON(&websocketMessage{
			Event: "candidate",
			Data:  string(candidateString),
		}); writeErr != nil {
			log.Println("error in writing candidate string to ws", writeErr.Error())
		}
	})

	// if the peerconnection state is changed below event will trigger,do  accordingly
	peerConnection.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		switch pcs {
		case webrtc.PeerConnectionStateFailed:
			log.Println("new connection state failed")
			if err := peerConnection.Close(); err != nil {
				log.Println("error in closing peer connection", err.Error())
			}
		case webrtc.PeerConnectionStateClosed:
			log.Println("new connection state closed")
			p.SignalPeerConnections()
		}
	})

	//whenever incoming video is received,just distribute it to all peers
	peerConnection.OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
		log.Println("peer connection on track function")
		trackLocal := p.AddTrack(tr)
		if trackLocal == nil {
			return
		}
		defer p.RemoveTrack(trackLocal)
		buf := make([]byte, 1500)
		for {
			i, _, err := tr.Read(buf)
			if err != nil {
				return
			}

			if _, err = trackLocal.Write(buf[:i]); err != nil {
				return
			}

		}
	})
	p.SignalPeerConnections()
	message := &websocketMessage{}
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			log.Println("Error in reading message via ws conn", err)
			return
		} else if err := json.Unmarshal(raw, &message); err != nil {
			log.Println("error in Unmarshal of the message read from connection", err.Error())
			return
		}

		switch message.Event {
		case "candidate":
			candidate := webrtc.ICECandidateInit{}
			if err := json.Unmarshal([]byte(message.Data), &candidate); err != nil {
				log.Println("error in Unmarshal of candidate data received", err.Error())
				return
			}

			if err := peerConnection.AddICECandidate(candidate); err != nil {
				log.Println("error in adding ice candidate", err.Error())
				return
			}
		case "answer":
			answer := webrtc.SessionDescription{}
			if err := json.Unmarshal([]byte(message.Data), &answer); err != nil {
				log.Println("error in unmarshal of data for answer", err.Error())
				return
			}

			if err := peerConnection.SetRemoteDescription(answer); err != nil {
				log.Println("error in setting remote SessionDescription", err.Error())
				return
			}
		}
	}
}
