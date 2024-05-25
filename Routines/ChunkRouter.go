package Routines

import (
	"sync"
	"time"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/gorilla/websocket"
)

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
		loggingChannel <- CreateLogMessage(zerolog.InfoLevel, "Here")
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
