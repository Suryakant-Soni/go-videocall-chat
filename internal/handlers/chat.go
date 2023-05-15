package handlers

import (
	"go-videocall-chat/pkg/chat"
	w "go-videocall-chat/pkg/webrtc"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

func RoomChat(c *fiber.Ctx) error {
	return c.Render("chat", fiber.Map{}, "layouts/main")
}

func RoomChatWebsocket(c *websocket.Conn) {
	log.Println("RoomChatWebsocket trigerred")
	// for the rooms we have uuid
	uuid := c.Params("uuid")
	if uuid == "" {
		return
	}
	w.RoomsLock.Lock()
	// check if the room already existed
	room := w.Rooms[uuid]
	w.RoomsLock.Unlock()
	if room == nil {
		return
	}
	if room.Hub == nil {
		return
	}
	chat.PeerChatConn(c.Conn, room.Hub)
}

func StreamChatWebsocket(c *websocket.Conn) {
	log.Println("StreamChatWebsocket trigerred")
	// for the streams we have suuid
	suuid := c.Params("suuid")
	if suuid == "" {
		return
	}
	w.RoomsLock.Lock()
	// check if the stream already existed
	if stream, ok := w.Streams[suuid]; ok {
		w.RoomsLock.Unlock()
		if stream.Hub == nil {
			hub := chat.NewHub()
			stream.Hub = hub
			go hub.Run()
		}
		// create a new client
		chat.PeerChatConn(c.Conn, stream.Hub)
		return
	}
	w.RoomsLock.Unlock()
}
