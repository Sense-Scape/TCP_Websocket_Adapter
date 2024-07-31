package Routines

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"strconv"
)

func HandleWSDataChunkTx(configJson map[string]interface{}, loggingChannel chan map[zerolog.Level]string, incomingDataChannel <-chan string, OutgoingReportingChannel chan string) {
	
	// Create websocket variables
	var port string

	// And then try parse the JSON string
	if WebSocketTxConfig, exists := configJson["WebSocketDataTxConfig"].(map[string]interface{}); exists {
		port = WebSocketTxConfig["Port"].(string)
		loggingChannel <- CreateLogMessage(zerolog.InfoLevel, "WebSocketDataTxConfig opening on port"  + port)
	} else {
		loggingChannel <- CreateLogMessage(zerolog.FatalLevel, "WebSocketDataTxConfig Config not found or not correct")
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

	go RunChunkRoutingRoutine(loggingChannel, incomingDataChannel, router, OutgoingReportingChannel)
	loggingChannel <- CreateLogMessage(zerolog.InfoLevel, "Starting http router")
	router.Run(":" + port)

}

func RunChunkRoutingRoutine(loggingChannel chan map[zerolog.Level]string, incomingDataChannel <-chan string, router *gin.Engine, OutgoingReportingChannel chan string) {
	
	chunkTypeRoutingMap := NewChunkTypeToChannelMap(loggingChannel, OutgoingReportingChannel)
	currentTime := time.Now()	

	for {

		bSendData := false
		var strJSONData string
		timeoutCh := time.After(5 * time.Millisecond)

		// Try get data while waiting for a timeout
		select {
		case strJSONData = <-incomingDataChannel:
			bSendData = true
		case <-timeoutCh:
		}

		// Then try send the data
		if bSendData {

			var JSONData map[string]interface{}
			
			if err := json.Unmarshal([]byte(strJSONData), &JSONData); err != nil {
				loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "Error unmarshaling JSON in routing routine:"+err.Error()+" - Got " + strJSONData)
				continue
			}

			// Then try forward the JSON data onwards
			// By first getting the root JSON Key (ChunkType)
			var chunkTypeStringKey string
			for key := range JSONData {
				chunkTypeStringKey = key
				break // We assume there's only one root key
			}

			// And checking if it exists and trying to route it
			if (chunkTypeStringKey == "SystemInfo") {
				OutgoingReportingChannel <- string(strJSONData)
			} else {
				chunkTypeRoutingMap.SendChunkToWebSocket(loggingChannel, chunkTypeStringKey, strJSONData, router)
			}
		}

		if time.Since(currentTime) > 1000*time.Millisecond {

			currentTime= time.Now()
			
			QueueLogMessage := SystemInfo{SystemStat:SystemStatistic{
				StatEnvironment: "TCP_WS_Adapter",
				StatName: "Routing_Output_Channel",
				StatStaus: strconv.Itoa(len(incomingDataChannel)) + "/" + strconv.Itoa(cap(incomingDataChannel)),
			}}
			
			data, _ := json.Marshal(QueueLogMessage)
			OutgoingReportingChannel <- string(data)
		}
	}
}

