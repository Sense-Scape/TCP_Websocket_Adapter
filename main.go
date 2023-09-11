package main

import (
	"net"
	"net/http"
	"os"
	"sync"

	ChunkCPPGoAdapter "github.com/Sense-Scape/Go_TCP_Websocket_Adapter/v2/ChunkCPPGoAdapter"
	"github.com/rs/zerolog"

	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func main() {

	dataChannel := make(chan string) // Create an integer channel

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	var wg sync.WaitGroup
	wg.Add(1)
	go handleConnectionWebSocketOutgoing(dataChannel, &wg)

	// Define the port to listen on
	port := "10005"

	// Create a TCP listener on the specified port
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatal().Msg("Error:" + err.Error())
		os.Exit(1)
	}
	defer listener.Close()

	logger.Info().Msg("TCP server is listening on port:" + port)

	// Accept incoming connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error().Msg("Error:" + err.Error())
			continue
		}
		go ChunkCPPGoAdapter.HandleConnectionTCPIncomingChunkTypes(dataChannel, conn)
	}

	wg.Wait()
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func handleConnectionWebSocketOutgoing(dataChannel <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger.Info().Msg("TransportLayerDataSize:")
	println("wohoo-----")

	router := gin.Default()

	// CORS middleware setup
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"http://localhost:5173/"} // Replace with your SvelteKit frontend URL

	corsGroup := router.Group("/private")
	corsGroup.Use(cors.New(corsConfig))

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
