package Routines

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

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
	loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "1:")
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

		
	loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "Registering on WebSocket: "+chunkTypeString)

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

///
///			ROUTINE SAFE MAP FUNCTIONS
///

/*
Routine safe map of key value pairs of strings and channels.
Each string corresponds to channel to send a chunk type to a
routine that shall handle that chunk
*/
type ChunkTypeToChannelMap struct {
	mu                  sync.Mutex               // Mutex to protect access to the map
	chunkTypeRoutingMap map[string](chan string) // Map of chunk type string and channel key value pairs
}

/*
Data string will be routed in the map given that chunk type key exists
*/
func (s *ChunkTypeToChannelMap) SendChunkToWebSocket(loggingChannel chan map[zerolog.Level]string, chunkTypeKey string, data string, router *gin.Engine) bool {

	// We first check if the channel exists
	// And wait to try get it
	chunkRoutingChannel, channelExists := s.TryGetChannel(chunkTypeKey)
	if channelExists {
		// and pass the data if it does
		chunkRoutingChannel <- data
		return channelExists
	} else {
		// or drop data and return false
		// operate on the router itself
		s.RegisterChunkOnWebSocket(loggingChannel, chunkTypeKey, router)
		return channelExists
	}
}

func (s *ChunkTypeToChannelMap) GetChannelData(chunkTypeKey string) (dataString string, success bool) {
	// We first check if the channel exists
	// And wait to try get it
	chunkRoutingChannel, channelExists := s.TryGetChannel(chunkTypeKey)
	if channelExists {
		// and pass the data if it does
		var data = <-chunkRoutingChannel
		success = true
		return data, success
	} else {
		// or drop data and return false
		success = false
		return "", success
	}

}

/*
We try and wait to get access to a channel. Once we get it, we can return it because channels
are routine safe. Concurrently accessing map requires mutex
*/
func (s *ChunkTypeToChannelMap) TryGetChannel(chunkType string) (extractedChannel chan string, exists bool) {
	for {
		var lockAcquired = make(chan struct{}, 1)

		go func() {
			s.mu.Lock()
			extractedChannel, exists = s.chunkTypeRoutingMap[chunkType]
			s.mu.Unlock()
			defer close(lockAcquired)
		}()

		select {
		case <-lockAcquired:
			// Lock was acquired
			return extractedChannel, exists
		case <-time.After(1 * time.Millisecond):
			// Lock was not acquired, sleep and retry
			time.Sleep(1 * time.Millisecond) // Sleep for 500 milliseconds, you can adjust the duration as needed.
		}
	}
}
