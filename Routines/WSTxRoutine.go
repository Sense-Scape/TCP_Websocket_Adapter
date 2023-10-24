package Routines

import (
	"encoding/json"
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
		if data, ok := WebSocketTxConfig["RegisteredChunks"].([]interface{}); ok {
			for _, item := range data {
				if chunkTypeString, isString := item.(string); isString {
					registeredChunks = append(registeredChunks, chunkTypeString)
				}
			}
		} else {
			loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "No chunks found to register in chunk map")
		}

	} else {
		loggingChannel <- CreateLogMessage(zerolog.FatalLevel, "WebSocketTxConfig Config not found or not correct")
		os.Exit(1)
		return
	}

	// Now we create a routine that will handle the receipt of JSON messages
	var chunkTypeChannelMap = RegisterChunkTypeMap(loggingChannel, registeredChunks)
	go RunChunkRoutingRoutine(loggingChannel, dataChannel, chunkTypeChannelMap)

	// Then run the HTTP router
	router := RegisterRouterWebSocketPaths(loggingChannel, chunkTypeChannelMap)
	loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "Starting http router")
	router.Run(":" + port)

}

func RegisterChunkTypeMap(loggingChannel chan map[zerolog.Level]string, registeredChunkTypes []string) map[string](chan string) {

	chunkTypeChannelMap := make(map[string](chan string))

	// Iterate through all chunk types and create a corresponding channel
	for _, chunkType := range registeredChunkTypes {
		loggingChannel <- CreateLogMessage(zerolog.InfoLevel, "Registering - "+chunkType+" - in Websocket routing map")
		chunkTypeChannelMap[chunkType] = make(chan string)
	}

	return chunkTypeChannelMap
}

func RunChunkRoutingRoutine(loggingChannel chan map[zerolog.Level]string, incomingDataChannel <-chan string, chunkTypeRoutingMap map[string](chan string)) {

	// Create an empty array (or slice) of strings
	var unregisteredChunkTypes []string

	// start up and handle JSON chunks
	for {

		// Unmarshal the JSON string into a map
		JSONDataString := <-incomingDataChannel
		var JSONData map[string]interface{}
		if err := json.Unmarshal([]byte(JSONDataString), &JSONData); err != nil {
			loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "Error unmarshaling JSON:"+err.Error())
			return
		} else {
			// Then try forward the JSON data onwards
			var chunkTypeStringKey string
			for key := range JSONData {
				chunkTypeStringKey = key
				break // We assume there's only one root key
			}

			outGoingChannel, exists := chunkTypeRoutingMap[chunkTypeStringKey]

			if exists {
				outGoingChannel <- JSONDataString
			} else {

				var chunkTypeAlreadyLogged = false
				// Now we see if we have logged that this is not supported already
				for _, LoggedChunkTypeString := range unregisteredChunkTypes {
					if LoggedChunkTypeString == chunkTypeStringKey {
						chunkTypeAlreadyLogged = true
					}
				}

				// Then log if we have not logged already
				if !chunkTypeAlreadyLogged {
					unregisteredChunkTypes = append(unregisteredChunkTypes, chunkTypeStringKey)
					loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "ChunkType - "+chunkTypeStringKey+" - not registered in routing map")
				}
			}

		}
	}
}

func RegisterRouterWebSocketPaths(loggingChannel chan map[zerolog.Level]string, JSONDataChannel map[string](chan string)) *gin.Engine {

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
			JSONDataString := <-JSONDataChannel["TimeChunk"]
			WebSocketConnection.WriteMessage(websocket.TextMessage, []byte(JSONDataString))
		}
	})

	return router
}
