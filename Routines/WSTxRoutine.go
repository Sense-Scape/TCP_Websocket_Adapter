package Routines

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func HandleWebSocketChunkTransmissions(configJson map[string]interface{}, loggingChannel chan map[zerolog.Level]string, dataChannel <-chan string) {

	// Create websocket variables
	var port string
	var registeredChunks []string

	// And then try parse the JSON string
	if WebSocketTxConfig, exists := configJson["WebSocketTxConfig"].(map[string]interface{}); exists {
		port = WebSocketTxConfig["Port"].(string)

		// Unmarshal the JSON data into the slice
		// And get registered chunktypes
		if data, ok := WebSocketTxConfig["WebSocketTxConfig"].([]interface{}); ok {
			for _, item := range data {
				if chunkTypeString, isString := item.(string); isString {
					registeredChunks = append(registeredChunks, chunkTypeString)
				}
			}
		}

	} else {
		loggingChannel <- CreateLogMessage(zerolog.FatalLevel, "WebSocketTxConfig Config not found or not correct")
		os.Exit(1)
		return
	}

	// Now we create a routine that will handle the receipt of JSON messages
	JSONDataChannel := make(chan string)
	chunkReceiverRoutine := func(JSONDataChannel chan string, dataChannel <-chan string) {
		// start up and handle JSON chunks
		for {
			// Get received JSON data and then Transmit it
			JSONDataString := <-dataChannel
			JSONDataChannel <- JSONDataString
		}
	}
	go chunkReceiverRoutine(JSONDataChannel, dataChannel)

	// Then run the HTTP router
	router := RegisterRouterPaths(loggingChannel, JSONDataChannel)
	loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "Starting http router")
	router.Run(":" + port)

}

func RegisterRouterPaths(loggingChannel chan map[zerolog.Level]string, JSONDataChannel <-chan string) *gin.Engine {

	router := gin.Default()

	// Allow all origins to connect
	// Note that is is not safe
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	router.GET("/DataTypes/TimeChunk", func(c *gin.Context) {
		// Upgrade the HTTP request into w websocket
		WebSocketConnection, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			// If it does not work log an error
			loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "Websocket error: "+err.Error())
			return
		}
		defer WebSocketConnection.Close()

		for {
			// Get received JSON data and then Transmit it
			JSONDataString := <-JSONDataChannel
			WebSocketConnection.WriteMessage(websocket.TextMessage, []byte(JSONDataString))
		}
	})

	return router
}
