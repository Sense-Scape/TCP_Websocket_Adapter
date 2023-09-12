package Routines

import (
	"os"
	"sync"
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

func HandleConnectionGoWebSocketOutgoing(dataChannel <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger.Info().Msg("TransportLayerDataSize:")
	println("wohoo-----")

	router := gin.Default()

	// // CORS middleware setup to allow local host access
	// // CORS: Cross-Origin Resource Sharing
	// corsConfig := cors.DefaultConfig()
	// corsConfig.AllowOrigins = []string{"http://localhost:5173/"}

	// // Now we specify that security is required for private
	// // Therefore other root paths do not require verification
	// corsGroup := router.Group("/private")
	// corsGroup.Use(cors.New(corsConfig))

	// Allow all origins to connect
	// Note that is is not safe
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	router.GET("/public", func(c *gin.Context) {

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logger.Error().Msg(err.Error())
			return
		}
		defer conn.Close()
		for {
			println("wohoo-----<")
			dataString := <-dataChannel
			println("wohoo-----<<")
			conn.WriteMessage(websocket.TextMessage, []byte(dataString))
			time.Sleep(time.Second)
			logger.Debug().Msg("Transmitting Websocket data")
		}
	})
	router.Run(":10010")

}
