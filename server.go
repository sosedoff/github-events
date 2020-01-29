package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type server struct {
	clients     map[*websocket.Conn]string
	clientsLock sync.Mutex
}

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		HandshakeTimeout: time.Second * 5,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

func (s *server) broadcast(key string, message interface{}) {
	s.clientsLock.Lock()
	for clientConn, clientKey := range s.clients {
		if clientKey == key {
			clientConn.WriteJSON(message)
		}
	}
	s.clientsLock.Unlock()
}

func (s *server) addClient(conn *websocket.Conn, key string) {
	s.clientsLock.Lock()
	s.clients[conn] = key
	s.clientsLock.Unlock()
}

func (s *server) removeClient(conn *websocket.Conn, key string) {
	s.clientsLock.Lock()
	delete(s.clients, conn)
	s.clientsLock.Unlock()
}

func (s *server) handleEvent(c *gin.Context) {
	key := c.Param("key")

	// Must be coming from Github
	if c.GetHeader("X-GitHub-Delivery") == "" {
		c.JSON(400, gin.H{"error": "X-GitHub-Delivery header is not set"})
		return
	}

	// Require an event name
	event := c.GetHeader("X-GitHub-Event")
	if event == "" {
		c.JSON(400, gin.H{"error": "X-GitHub-Event header is not set"})
		return
	}

	var msg json.RawMessage

	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(&msg); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if len(s.clients) > 0 {
		go s.broadcast(key, gin.H{"event": event, "payload": msg})
	}

	c.JSON(200, gin.H{"accepted": true})
}

func (s *server) handleListen(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("connection upgrade error:", err)
		return
	}
	defer conn.Close()

	key := c.Param("key")

	s.addClient(conn, key)
	defer s.removeClient(conn, key)

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			log.Println("read error:", err)
			break
		}
	}
}

func getListenAddr(key string, port string) string {
	listenPort := os.Getenv(key)
	if listenPort == "" {
		listenPort = port
	}
	return "0.0.0.0:" + listenPort
}

func newServer() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	srv := server{
		clients:     map[*websocket.Conn]string{},
		clientsLock: sync.Mutex{},
	}

	router := gin.Default()
	router.POST("/:key", srv.handleEvent)
	router.GET("/:key", srv.handleListen)

	return router
}
