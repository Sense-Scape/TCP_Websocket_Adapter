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
	var registeredChunks []string

	// And then try parse the JSON string
	if WebSocketTxConfig, exists := configJson["WebSocketTxConfig"].(map[string]interface{}); exists {
		port = WebSocketTxConfig["Port"].(string)

		// Unmarshal the JSON data into the slice
		// And get registered Chunk Types
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

	// Now we create a routine that will handle the reception
	// And retransmission of JSON documents
	var chunkTypeChannelMap = RegisterChunkTypeMap(loggingChannel, registeredChunks)
	go RunChunkRoutingRoutine(loggingChannel, incomingDataChannel, chunkTypeChannelMap)

	// Then we run the HTTP router
	router := RegisterRouterWebSocketPaths(loggingChannel, chunkTypeChannelMap)
	loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "Starting http router")
	router.Run(":" + port)

}

/*
Take the list of registered chunk types and create a channel
corresponding to each one
*/
func RegisterChunkTypeMap(loggingChannel chan map[zerolog.Level]string, registeredChunkTypes []string) *SafeChannelMap {

	chunkTypeChannelMap := make(map[string](chan string))

	// Iterate through all chunk types and create a corresponding channel
	for _, chunkType := range registeredChunkTypes {
		loggingChannel <- CreateLogMessage(zerolog.InfoLevel, "Registering - "+chunkType+" - in Websocket routing map")
		chunkTypeChannelMap[chunkType] = make(chan string)
	}

	safeChannelMap := new(SafeChannelMap)
	safeChannelMap.chunkTypeRoutingMap = chunkTypeChannelMap

	return safeChannelMap
}

func RunChunkRoutingRoutine(loggingChannel chan map[zerolog.Level]string, incomingDataChannel <-chan string, chunkTypeRoutingMap *SafeChannelMap) {

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
			// By first getting the root JSON Key (ChunkType)
			var chunkTypeStringKey string
			for key := range JSONData {
				chunkTypeStringKey = key
				break // We assume there's only one root key
			}

			// And checking if it exists and trying to route it
			sentSuccessfully := chunkTypeRoutingMap.SendSafeChannelMapData(chunkTypeStringKey, JSONDataString)
			if !sentSuccessfully {
				// We did not send data so we
				// now we see if we have logged
				// that the channel does not exist
				var chunkTypeAlreadyLogged = false
				for _, LoggedChunkTypeString := range unregisteredChunkTypes {
					if LoggedChunkTypeString == chunkTypeStringKey {
						chunkTypeAlreadyLogged = true
					}
				}

				// And log if we have not logged already
				if !chunkTypeAlreadyLogged {
					unregisteredChunkTypes = append(unregisteredChunkTypes, chunkTypeStringKey)
					loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "ChunkType - "+chunkTypeStringKey+" - not registered in routing map")
				}
			}
		}
	}
}

func RegisterRouterWebSocketPaths(loggingChannel chan map[zerolog.Level]string, chunkTypeChannelMap *SafeChannelMap) *gin.Engine {

	router := gin.Default()

	// Allow all origins to connect
	// Note that is is not safe
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	router.GET("/DataTypes/TimeChunk", func(c *gin.Context) {
		// Upgrade the HTTP request into a websocket
		WebSocketConnection, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			// If it does not work log an error
			loggingChannel <- CreateLogMessage(zerolog.InfoLevel, "Websocket error: "+err.Error())
			return
		}
		defer WebSocketConnection.Close()

		loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "TimeChunk websocket connection connected")

		// Then start up
		incomingJSONChannel, ChannelExists := chunkTypeChannelMap.TryGetChannel("TimeChunk")
		if ChannelExists {
			for {
				JSONDataString := <-incomingJSONChannel
				WebSocketConnection.WriteMessage(websocket.TextMessage, []byte(JSONDataString))
			}
		} else {
			loggingChannel <- CreateLogMessage(zerolog.InfoLevel, "Websocket error: TimeChunk channel does not exist")
		}

	})

	return router
}

/*
Routine safe map of key value pairs of strings and channels.
Each string corresponds to channel to send a chunk type to a
routine that shall handle that chunk
*/
type SafeChannelMap struct {
	mu                  sync.Mutex               // Mutex to protect access to the map
	chunkTypeRoutingMap map[string](chan string) // Map of chunk type string and channel key value pairs
}

/*
Data string will be routed in the map given that chunk type key exists
*/
func (s *SafeChannelMap) SendSafeChannelMapData(chunkTypeKey string, data string) bool {

	// We first check if the channel exists
	// And wait to try get it
	chunkRoutingChannel, channelExists := s.TryGetChannel(chunkTypeKey)
	if channelExists {
		// and pass the data if it does
		chunkRoutingChannel <- data
		return channelExists
	} else {
		// or drop data and return false
		return channelExists
	}
}

/*
We try and wait to get access to a channel. Once we get it, we can return it because channels
are routine safe. Concurrently accessing map requires mutex
*/
func (s *SafeChannelMap) TryGetChannel(chunkType string) (extractedChannel chan string, exists bool) {
	var lockAcquired chan struct{}

	for {
		lockAcquired = make(chan struct{})

		go func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			extractedChannel, exists = s.chunkTypeRoutingMap[chunkType]
			close(lockAcquired)
		}()

		select {
		case <-lockAcquired:
			// Lock was acquired
			return extractedChannel, exists
		case <-time.After(5 * time.Millisecond):
			// Lock was not acquired, sleep and retry
			close(lockAcquired)
			time.Sleep(5 * time.Millisecond) // Sleep for 500 milliseconds, you can adjust the duration as needed.
		}
	}
}

func (s *SafeChannelMap) ReceiveSafeChannelMapData(chunkType string) (dataString string) {
	var lockAcquired chan struct{}

	for {
		lockAcquired = make(chan struct{})

		go func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			dataString = <-s.chunkTypeRoutingMap[chunkType]
			close(lockAcquired)
		}()

		select {
		case <-lockAcquired:
			// Lock was acquired
			close(lockAcquired)
			return dataString
		case <-time.After(5 * time.Millisecond):
			// Lock was not acquired, sleep and retry
			close(lockAcquired)
			time.Sleep(5 * time.Millisecond) // Sleep for 500 milliseconds, you can adjust the duration as needed.
		}
	}
}
