package Routines

import (
	"sync"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/gorilla/websocket"
	"time"
	"strconv"
	"sync/atomic"
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
	loggingOutputChannel 	chan map[zerolog.Level]string	// Channel to stream logging messages
	reportingOutputChannel 	chan string	// Channel to stream Reporting messages
	chunkTypeRoutingMap 	map[string](chan string) 		// Map of chunk type string and channel key value pairs
	mu                  	sync.Mutex               		// Mutex to protect access to the map
}

func NewChunkTypeToChannelMap(loggingOutputChannel 	chan map[zerolog.Level]string, reportingOutputChannel chan string) *ChunkTypeToChannelMap {
    p := new(ChunkTypeToChannelMap)
    p.loggingOutputChannel = loggingOutputChannel
	p.reportingOutputChannel = reportingOutputChannel 
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
	if !channelExists {
		return "", false
	}

	var timeoutCh = time.After(250 * time.Millisecond)

	select {
	case data := <-chunkRoutingChannel:
		return data, true
	case <-timeoutCh:
		return "", false
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

func (s *ChunkTypeToChannelMap) GetChannelLengthAndCapacity(chunkTypeString string) (length int, capacity int, exists bool) {
	var channel,exits = s.TryGetChannel(chunkTypeString)

	if !exits {
		return -1, -1, exits
	}

	return len(channel), cap(channel), exits
}

func (s *ChunkTypeToChannelMap)RegisterChunkOnWebSocket(loggingChannel chan map[zerolog.Level]string, chunkTypeString string, router *gin.Engine) {

		
	loggingChannel <- CreateLogMessage(zerolog.InfoLevel, "Registering on WebSocket: "+chunkTypeString)

	// if we have not started using the map yet then intialise it
	if s.chunkTypeRoutingMap == nil {
		chunkTypeChannelMap := make(map[string]chan string)
		s.chunkTypeRoutingMap = chunkTypeChannelMap
	}

	s.chunkTypeRoutingMap[chunkTypeString] = make(chan string, 1000)

	// When you get this HTTP request open the websocket
	// This permenantly add this to the http 
	// Router as we are using a reference
	router.GET("/DataTypes/"+chunkTypeString, func(c *gin.Context) {

            // Upgrade the HTTP request into a websocket
			loggingChannel <- CreateLogMessage(zerolog.InfoLevel, "Client calling for upgrade on /DataTypes/"+chunkTypeString)
            WebSocketConnection, err := upgrader.Upgrade(c.Writer, c.Request, nil)
			defer WebSocketConnection.Close()

            if err != nil {
                loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "Error upgrading to WebSocket:"+ err.Error())
				return
			}

			// Spin up Routines to manage this websocket upgrade request
			// When this socket is closed all management of this queue is
			// Stopped leading it to grow to its max capacity and lock up


			var AtomicWebsocketClosed atomic.Bool // Atomic integer used as a flag (0: false, 1: true)
			AtomicWebsocketClosed.Store(false)

			var wg sync.WaitGroup
			wg.Add(2)
			go s.HandleReceivedSignals(loggingChannel, WebSocketConnection, &wg, &AtomicWebsocketClosed);
			go s.HandleSignalTransmissions(loggingChannel ,WebSocketConnection, chunkTypeString, &wg, &AtomicWebsocketClosed )
			wg.Wait()

			loggingChannel <- CreateLogMessage(zerolog.InfoLevel, chunkTypeString + " Routine shut down")

	})
}

func (s *ChunkTypeToChannelMap)HandleReceivedSignals(loggingChannel chan map[zerolog.Level]string, WebSocketConnection *websocket.Conn, wg *sync.WaitGroup, AtomicWebsocketClosed *atomic.Bool) {

	defer wg.Done()
	
	for{
		// We now just wait for the close message
		_, _, err := WebSocketConnection.ReadMessage()
		if err != nil {
			loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "Issue reading message from WebSocket:" + err.Error())
			AtomicWebsocketClosed.Store(true)
			break;
		}
	}
}

func (s *ChunkTypeToChannelMap)HandleSignalTransmissions(loggingChannel chan map[zerolog.Level]string, WebSocketConnection *websocket.Conn, chunkTypeString string, wg *sync.WaitGroup, AtomicWebsocketClosed *atomic.Bool) {

	defer wg.Done()
	currentTime := time.Now()
	
	for {
		// Unmarshal the JSON string into a map
		var bChannelExists = false
		var strJSONData string

		var chstrJSONData = make(chan string, 1)
		var chbCannelExists = make(chan bool, 1)
		
		// Launch a goroutine to try fetch data
		go func() {
			strJSONDataTemp, bChannelExistsTmp := s.GetChannelData(chunkTypeString)
			chbCannelExists <- bChannelExistsTmp
			chstrJSONData <- strJSONDataTemp
		}()

		// While also limiting this with a timeout procedure
		var chTimeout = time.After(250 * time.Millisecond)

		// And see which finihsed first
		select {
		case bChannelExists = <-chbCannelExists:
			strJSONData = <- chstrJSONData
			loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "1")
		case <-chTimeout:
			// And continure if a timeout occurred
			continue
		}

		// When we have data check the client has not closed out beautiful, stunning and special connection
		if AtomicWebsocketClosed.Load() {
			loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "RX Websocket closed, exiting write routine")
			break
		}
		
		// If the connection is open check we successfully got access to the channel and transmit data
		if bChannelExists {
			err := WebSocketConnection.WriteMessage(websocket.TextMessage, []byte(strJSONData))
			if err != nil {
				loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "Issue writing message to WebSocket:"+ err.Error())
				AtomicWebsocketClosed.Store(true)
				break
			}
		}

		// Then check if we should report the length of this chunks data channel
		if time.Since(currentTime) > 1000*time.Millisecond {

			var len, cap, _ = s.GetChannelLengthAndCapacity(chunkTypeString)
			currentTime= time.Now()

			// Create the reporting message
			QueueLogMessage := SystemInfo{SystemStat:SystemStatistic{
				StatEnvironment: "TCP_WS_Adapter",
				StatName: chunkTypeString + "_Channel",
				StatStaus: strconv.Itoa(len) + "/" + strconv.Itoa(cap),
			}}
			
			// And send it to the dedicated reporting routine
			data, _ := json.Marshal(QueueLogMessage)
			s.reportingOutputChannel <- string(data)
		}
	}
}