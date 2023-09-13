package Routines

import (
	"os"
	"time"

	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func HandleWebSocketTimeChunkTransmissions(dataChannel <-chan string) {
	defer wg.Done()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	router := gin.Default()

	// Allow all origins to connect
	// Note that is is not safe
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	router.GET("/DataTypes/TimeChunk", func(c *gin.Context) {

		WebSocketConnection, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logger.Error().Msg(err.Error())
			return
		}
		defer WebSocketConnection.Close()
		for {
			// Get received JSON data and then
			// Transmit it and then have a nap
			JSONDataString := <-dataChannel
			WebSocketConnection.WriteMessage(websocket.TextMessage, []byte(JSONDataString))
			time.Sleep(time.Second)
		}
	})
	router.Run(":10010")
}
