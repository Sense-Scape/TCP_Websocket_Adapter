package Routines

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Sense-Scape/Go_TCP_Websocket_Adapter/v2/ChunkRouter"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func HandleWebSocketChunkTransmissions(configJson map[string]interface{}, loggingChannel chan map[zerolog.Level]string, incomingDataChannel <-chan string) {

	// Create websocket variables
	var port string

	// And then try parse the JSON string
	if WebSocketTxConfig, exists := configJson["WebSocketTxConfig"].(map[string]interface{}); exists {
		port = WebSocketTxConfig["Port"].(string)
	} else {
		loggingChannel <- CreateLogMessage(zerolog.FatalLevel, "WebSocketTxConfig Config not found or not correct")
		os.Exit(1)
		return
	}

	// Then we run the HTTP router
	router := gin.Default()
	// Allow all origins to connect
	// Note that is is not safe
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	go RunChunkRoutingRoutine(loggingChannel, incomingDataChannel, router)
	loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "Starting http router")
	router.Run(":" + port)

}

func RunChunkRoutingRoutine(loggingChannel chan map[zerolog.Level]string, incomingDataChannel <-chan string, router *gin.Engine) {
	
	chunkTypeRoutingMap := new(ChunkTypeToChannelMap)
	// start up and handle JSON chunks
	for {

		// Unmarshal the JSON string into a map
		JSONDataString := <-incomingDataChannel
		var JSONData map[string]interface{}
		if err := json.Unmarshal([]byte(JSONDataString), &JSONData); err != nil {
			loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "Error unmarshaling JSON in routing routine:"+err.Error())
			// loggingChannel <- CreateLogMessage(zerolog.DebugLevel, "Received JSON as follows { "+JSONDataString+" }")
			// return
		} else {
			// Then try forward the JSON data onwards
			// By first getting the root JSON Key (ChunkType)
			var chunkTypeStringKey string

			for key := range JSONData {
				chunkTypeStringKey = key
				break // We assume there's only one root key
			}

			// And checking if it exists and trying to route it
			sentSuccessfully := chunkTypeRoutingMap.SendChunkToWebSocket(loggingChannel, chunkTypeStringKey, JSONDataString, router)
			if !sentSuccessfully {
					loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "ChunkType - "+chunkTypeStringKey+" - newly registered in routing map")
			}
		}
	}
}

func (s *ChunkTypeToChannelMap)RegisterChunkOnWebSocket(loggingChannel chan map[zerolog.Level]string, chunkTypeString string, router *gin.Engine) {

		
	loggingChannel <- CreateLogMessage(zerolog.InfoLevel, "Registering on WebSocket: "+chunkTypeString)

	// if we have not started using the map yet then intialise it
	if s.chunkTypeRoutingMap == nil {
		chunkTypeChannelMap := make(map[string]chan string)
		s.chunkTypeRoutingMap = chunkTypeChannelMap
	}

	s.chunkTypeRoutingMap[chunkTypeString] = make(chan string, 100)

	 router.GET("/DataTypes/"+chunkTypeString, func(c *gin.Context) {
		// Upgrade the HTTP request into a websocket
		WebSocketConnection, _ := upgrader.Upgrade(c.Writer, c.Request, nil)

		defer WebSocketConnection.Close()

		currentTime := time.Now()
		lastTime := currentTime

		// Then start up
		var dataString, _ = s.GetChannelData(chunkTypeString)

		WebSocketConnection.WriteMessage(websocket.TextMessage, []byte(dataString))
		for {

			currentTime = time.Now()
			timeDiff := currentTime.Sub(lastTime)

			var dataString, _ = s.GetChannelData(chunkTypeString)

			// Rate limiting
			if timeDiff > (time.Millisecond * 1) {
				WebSocketConnection.WriteMessage(websocket.TextMessage, []byte(dataString))
				lastTime = currentTime
			}
		}
	})
}
