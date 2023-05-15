package server

import (
	"flag"
	"go-videocall-chat/internal/handlers"
	w "go-videocall-chat/pkg/webrtc"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html"
	"github.com/gofiber/websocket/v2"
)

var (
	addr = flag.String("addr", ":"+os.Getenv("PORT"), "")
	cert = flag.String("cert", "", "")
	key  = flag.String("key", "", "")
)

func Run() error {

	flag.Parse()
	if *addr == ":" {
		*addr = "8080"
	}

	htmlEngine := html.New("./views", ".html")
	app := fiber.New(fiber.Config{Views: htmlEngine})
	app.Use(logger.New())
	app.Use(cors.New())
	//welcome file is displayed
	app.Get("/", handlers.Welcome)
	// it will simply create a new guid for room and append it in the create room url and hit back
	app.Get("/room/create", handlers.RoomCreate)
	
	app.Get("/room/:uuid", handlers.Room)
	app.Get("/room/:uuid/websocket", websocket.New(handlers.RoomWebsocket, websocket.Config{
		HandshakeTimeout: time.Second * 10,
	}))
	app.Get("/room/:uuid/chat", handlers.RoomChat)
	app.Get("/room/:uuid/chat/websocket", websocket.New(handlers.RoomChatWebsocket))
	app.Get("/room/:uuid/viewer/websocket", websocket.New(handlers.RoomViewerWebsocket))
	app.Get("/stream/:ssuid", handlers.Stream)
	app.Get("/stream/:ssuid/websocket", websocket.New(handlers.StreamWebsocket, websocket.Config{
		HandshakeTimeout: 10 * time.Second,
	}))
	app.Get("/stream/:ssuid/chat/websocket", websocket.New(handlers.StreamChatWebsocket))
	app.Get("/stream/:ssuid/viewer/websocket", websocket.New(handlers.StreamViewerWebsocket))
	app.Static("/", "./assets")

	w.Rooms = make(map[string]*w.Room)
	w.Streams = make(map[string]*w.Room)

	go dispatchKeyFrames()

	if *cert != "" {
		return app.ListenTLS(*addr, *cert, *key)
	}
	return app.Listen(*addr)
}

func dispatchKeyFrames() {
	for range time.NewTicker(3 * time.Second).C {
		for _, room := range w.Rooms {
			room.Peers.DispatchKeyFrames()
		}
	}
}
