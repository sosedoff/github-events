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

var (
	defaultOriginHandler = func(r *http.Request) bool {
		return true
	}

	upgrader = websocket.Upgrader{
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		HandshakeTimeout: time.Second * 5,
		CheckOrigin:      defaultOriginHandler,
	}

	listeners     = map[string][]*websocket.Conn{}
	listenersLock = sync.Mutex{}
)

func getListenAddr(port string) string {
	listenPort := os.Getenv("PORT")
	if listenPort == "" {
		listenPort = port
	}
	return "0.0.0.0:" + listenPort
}

func handleEvent(c *gin.Context) {
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

	if len(listeners) == 0 {
		return
	}

	go func() {
		msg := gin.H{"event": event, "payload": msg}

		listenersLock.Lock()
		for _, l := range listeners[key] {
			l.WriteJSON(msg)
		}
		listenersLock.Unlock()
	}()

	c.JSON(200, gin.H{"accepted": true})
}

func handleListen(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("connection upgrade error:", err)
		return
	}
	defer conn.Close()

	key := c.Param("key")

	listenersLock.Lock()
	if listeners[key] == nil {
		listeners[key] = []*websocket.Conn{conn}
	} else {
		listeners[key] = append(listeners[key], conn)
	}
	listenersLock.Unlock()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			log.Println("read error:", err)
			break
		}
	}

	listenersLock.Lock()
	newlisteners := []*websocket.Conn{}
	for _, l := range listeners[key] {
		if l != conn {
			newlisteners = append(newlisteners, l)
		}
	}
	listeners[key] = newlisteners
	listenersLock.Unlock()
}

func newServer() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	router.POST("/:key", handleEvent)
	router.GET("/:key", handleListen)
	return router
}
