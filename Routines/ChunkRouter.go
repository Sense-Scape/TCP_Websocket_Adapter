package Routines

import (
	"sync"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/gorilla/websocket"
	"time"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  8096,
	WriteBufferSize: 8096,
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
	loggingOutputChannel 	chan map[zerolog.Level]string	// Channel to stream loggin messages
	chunkTypeRoutingMap 	map[string](chan string) 		// Map of chunk type string and channel key value pairs
	mu                  	sync.Mutex               		// Mutex to protect access to the map
	routineWaitGroup		sync.WaitGroup
}

func NewChunkTypeToChannelMap(loggingOutputChannel 	chan map[zerolog.Level]string) *ChunkTypeToChannelMap {
    p := new(ChunkTypeToChannelMap)
    p.loggingOutputChannel = loggingOutputChannel
    return p
}
/*
Data string will be routed in the map given that chunk type key exists
*/
func (s *ChunkTypeToChannelMap) SendChunkToWebSocket(loggingChannel chan map[zerolog.Level]string, chunkTypeKey string, data string, router *gin.Engine) {

	// We first check if the channel exists
	// And wait to try get it
	chunkRoutingChannel, channelExists := s.TryGetChannel(chunkTypeKey)
	if channelExists {
		// and try pass the data if it does if there is space in the queue
		if len(chunkRoutingChannel) >= cap(chunkRoutingChannel) {
			
			// Note: One should see issues in the status messages. It may be useful to log this but the current
			// Implementation does not gaurentee that the UI will service all queues which will lead most of them
			// To be overloaded a lot of the time as only the loaded page will service a particular websocket

			//loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "ChunkType - "+chunkTypeKey+" - Routing queue overflowwing")
			return
		}
		chunkRoutingChannel <- data

	} else {
		// If it does not set up a weboscket connection
		// To manage connections for this chunk type
		s.RegisterChunkOnWebSocket(loggingChannel, chunkTypeKey, router)
		loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "ChunkType - "+chunkTypeKey+" - registered for routing")
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

		// Spin this lambda up and try get the channel
		go func() {
			s.mu.Lock()
			extractedChannel, exists = s.chunkTypeRoutingMap[chunkType]
			s.mu.Unlock()
			defer close(lockAcquired)
		}()

		// Wait here until lock aquired channel is closed
		select {
		case <-lockAcquired:
			// Lock was acquired
			return extractedChannel, exists
		case <-time.After(1 * time.Millisecond):
			// Lock was not acquired, sleep and retry
			time.Sleep(1 * time.Millisecond)
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

	// When you get this HTTP request open the websocket
	// This permenantly add this to the http 
	// Router as we are using a reference
	router.GET("/DataTypes/"+chunkTypeString, func(c *gin.Context) {

            // Upgrade the HTTP request into a websocket
            WebSocketConnection, err := upgrader.Upgrade(c.Writer, c.Request, nil)
			defer WebSocketConnection.Close()

            if err != nil {
                loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "Error upgrading to WebSocket:"+ err.Error())
				return
			}

			// Spin up Routines to manage this websocket upgrade request
			// When this socket is closed all management of this queue is
			// Stopped leading it to grow to its max capacity and lock up
			s.routineWaitGroup.Add(2)
			go s.HandleReceivedSignals(loggingChannel, WebSocketConnection);
			go s.HandleSignalTransmissions(loggingChannel ,WebSocketConnection, chunkTypeString )
			s.routineWaitGroup.Wait()

	})
}

func (s *ChunkTypeToChannelMap)HandleReceivedSignals(loggingChannel chan map[zerolog.Level]string, WebSocketConnection *websocket.Conn) {

	defer s.routineWaitGroup.Done()
	
	for{
		// We now just wait for the close message
		_, _, err := WebSocketConnection.ReadMessage()
		if err != nil {
			loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "Issue reading message to WebSocket:"+ err.Error())
			WebSocketConnection.Close()
			break;
		}
	}
}

func (s *ChunkTypeToChannelMap)HandleSignalTransmissions(loggingChannel chan map[zerolog.Level]string, WebSocketConnection *websocket.Conn, chunkTypeString string) {

	defer s.routineWaitGroup.Done()

	for {
		// Now we get the data to transmit on the websocket
		var dataString, _ = s.GetChannelData(chunkTypeString)
		err := WebSocketConnection.WriteMessage(websocket.TextMessage, []byte(dataString))
		if err != nil {
			loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "Issue writing message to WebSocket:"+ err.Error())
			WebSocketConnection.Close()
			break
		}
	}
}