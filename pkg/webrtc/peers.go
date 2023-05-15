package webrtc

import (
	"encoding/json"
	"go-videocall-chat/pkg/chat"
	"log"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

var (
	RoomsLock sync.RWMutex
	Rooms     map[string]*Room
	Streams   map[string]*Room
)

var turnConfig = webrtc.Configuration{
	ICETransportPolicy: webrtc.ICETransportPolicyRelay,
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:turn.localhost:3478"},
		},
		{
			URLs:           []string{"turn:turn.localhost:3478"},
			Username:       "suryakant",
			Credential:     "soni",
			CredentialType: webrtc.ICECredentialTypePassword,
		},
	},
}

type Room struct {
	Peers *Peers //peers which there in the room
	Hub   *chat.Hub
}

// TODO change to peer instead of peers at all place
type Peers struct {
	ListLock    sync.RWMutex
	Connections []PeerConnectionState // the multiple connections which peers will have
	TrackLocals map[string]*webrtc.TrackLocalStaticRTP
}

type PeerConnectionState struct {
	PeerConnection *webrtc.PeerConnection
	Websocket      *ThreadSafeWriter
}

type ThreadSafeWriter struct {
	Conn  *websocket.Conn
	Mutex sync.Mutex
}

func (t *ThreadSafeWriter) WriteJSON(v interface{}) error {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()
	return t.Conn.WriteJSON(v)
}

func (p *Peers) AddTrack(t *webrtc.TrackRemote) *webrtc.TrackLocalStaticRTP {
	p.ListLock.Lock()
	defer func() {
		p.ListLock.Unlock()
		p.SignalPeerConnections()
	}()

	trackLocal, err := webrtc.NewTrackLocalStaticRTP(t.Codec().RTPCodecCapability, t.ID(), t.StreamID())
	if err != nil {
		log.Println("error in creating track local", err.Error())
		return nil
	}
	p.TrackLocals[t.ID()] = trackLocal
	return trackLocal
}

func (p *Peers) RemoveTrack(t *webrtc.TrackLocalStaticRTP) {
	p.ListLock.Lock()
	defer func() {
		p.ListLock.Unlock()
		p.SignalPeerConnections()
	}()

	delete(p.TrackLocals, t.ID())
}

// used for signalling peer for the peer connection
func (p *Peers) SignalPeerConnections() {
	p.ListLock.Lock()
	defer func() {
		p.ListLock.Unlock()
		p.DispatchKeyFrames()
	}()
	attemptSync := func() (tryAgain bool) {
		log.Println("connections before attemp synic", p.Connections)
		for i := range p.Connections {
			if p.Connections[i].PeerConnection.ConnectionState() == webrtc.PeerConnectionStateClosed {
				log.Println("connection in closed state", p.Connections[i])
				p.Connections = append(p.Connections[:i], p.Connections[i+1:]...)
				log.Println("connections now", p.Connections)
				// since the connection is removed attempt sync once again with remaining connections
				return true
			}

			existingSenders := map[string]bool{}
			for _, sender := range p.Connections[i].PeerConnection.GetSenders() {
				if sender.Track() == nil {
					continue
				}
				existingSenders[sender.Track().ID()] = true

				if _, ok := p.TrackLocals[sender.Track().ID()]; !ok {
					if err := p.Connections[i].PeerConnection.RemoveTrack(sender); err != nil {
						log.Println("error in removng track", err.Error())
						return true
					}
				}
			}
			for _, receiver := range p.Connections[i].PeerConnection.GetReceivers() {
				if receiver.Track == nil {
					continue
				}
				existingSenders[receiver.Track().ID()] = true
			}

			for trackId := range p.TrackLocals {
				if _, ok := existingSenders[trackId]; ok {
					if _, err := p.Connections[i].PeerConnection.AddTrack(p.TrackLocals[trackId]); err != nil {
						log.Println("error in adding track", err.Error())
						return true
					}
				}
			}

			offer, err := p.Connections[i].PeerConnection.CreateOffer(nil)
			if err != nil {
				log.Println("error in creating offer", err.Error())
				return true
			}

			if err = p.Connections[i].PeerConnection.SetLocalDescription(offer); err != nil {
				return true
			}

			offerString, err := json.Marshal(offer)
			if err != nil {
				log.Println("error in marshalling offer json", err.Error())
				return true
			}

			if err = p.Connections[i].Websocket.WriteJSON(&websocketMessage{
				Event: "offer",
				Data:  string(offerString),
			}); err != nil {
				return true
			}
		}
		return
	}
	for syncAttempt := 0; ; syncAttempt++ {
		if syncAttempt == 25 {
			go func() {
				time.Sleep(time.Second * 3)
				p.SignalPeerConnections()
			}()
			return
		}

		if !attemptSync() {
			break
		}
	}
}

func (p *Peers) DispatchKeyFrames() {
	p.ListLock.Lock()
	defer p.ListLock.Unlock()

	for i := range p.Connections {
		for _, receiver := range p.Connections[i].PeerConnection.GetReceivers() {
			if receiver.Track() == nil {
				continue
			}

			if receiver.Track() == nil {
				continue
			}

			_ = p.Connections[i].PeerConnection.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{
					MediaSSRC: uint32(receiver.Track().SSRC()),
				},
			})
		}
	}
}

type websocketMessage struct {
	Event string `json:"event"`
	Data  string `json:"event"`
}
