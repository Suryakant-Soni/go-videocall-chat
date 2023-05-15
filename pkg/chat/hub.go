package chat

type Hub struct {
	clients    map[*Client]bool //clients are the browsers client which connects to the application
	broadcast  chan []byte      // the message which is sent by someone i,e, from any client
	register   chan *Client     // when any browser connects
	unregister chan *Client     // when any browser disconnects or goes away
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		// when any client is starting to enter
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				// select {
				// case client.Send <- message:
				// default:
				// 	close(client.Send)
				// 	delete(h.clients, client)
				// }
				// TODO: need to check if the default case is really needed means if the channel is blocked then do we really need to delete the channel 
				client.Send <- message
			}
		}
	}
}
