package chat

import (
	"bytes"
	"log"
	"time"

	"github.com/fasthttp/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (9 * pongWait) / 10
	maxMessageSize = 512
)

var (
	newline = []byte("\n")
	space   = []byte(" ")
)

var upgrader = websocket.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Client struct {
	Hub  *Hub
	Conn *websocket.Conn
	Send chan []byte
}

func (c *Client) readPump() {
	// after reading from a client we need to unregister the client and close the websocket connection to that client
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(maxMessageSize)
	// socket will be aavailable for next 1 min to read from the peers
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(appData string) error {
		// once the pong acknowledge ment is received again se the deadline to for the socket to remain open to 60 secs
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		// reading message from the socket
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		c.Hub.broadcast <- message
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				return
			}
			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			// TODO need to check if the below write statement can come under the loop
			w.Write(message)
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.Send)
			}
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			// the deadline will be extended after the ticker is triggered
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			// we will do a nil test write and if the write action is giving some error we will return from the select
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func PeerChatConn(c *websocket.Conn, hub *Hub) {
	// created a new client using websocket connection
	client := &Client{
		Hub:  hub,
		Conn: c,
		Send: make(chan []byte),
	}
	// once the client is created registering it by sending in register channel of client which is then read by Run Hub routine
	client.Hub.register <- client

	go client.writePump()
	client.readPump()
}
