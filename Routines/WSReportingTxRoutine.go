package Routines

import (
	"encoding/json"
	"net/http"
	"os"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)


func HandleWSReportingTx(configJson map[string]interface{}, routineCompleteChannel chan bool, loggingChannel chan map[zerolog.Level]string, incomingDataChannel <-chan string) {

	// Create websocket variables
	var port string

	// And then try parse the JSON string
	if WebSocketTxConfig, exists := configJson["WebSocketReportingTxConfig"].(map[string]interface{}); exists {
		port = WebSocketTxConfig["Port"].(string)
		loggingChannel <- CreateLogMessage(zerolog.InfoLevel, "WebSocketReportingTxConfig opening on port"  + port)
	} else {
		
		loggingChannel <- CreateLogMessage(zerolog.FatalLevel, "WebSocketReportingTxConfig Config not found or not correct")
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

	go RunReportingRoutine(loggingChannel, routineCompleteChannel, incomingDataChannel, router)
	loggingChannel <- CreateLogMessage(zerolog.InfoLevel, "Starting http router")
	router.Run(":" + port)

}

func RunReportingRoutine(loggingChannel chan map[zerolog.Level]string, routineCompleteChannel chan bool, incomingDataChannel <-chan string, router *gin.Engine) {

	chunkTypeRoutingMap := NewChunkTypeToChannelMap(loggingChannel)

	for {

		// Unmarshal the JSON string into a map
		JSONDataString := <-incomingDataChannel
		var JSONData map[string]interface{}

		// Try convert the JSON doc to a string
		if err := json.Unmarshal([]byte(JSONDataString), &JSONData); err != nil {
			// If it fails, then skip to next interation
			loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "Error unmarshaling JSON in routing routine:"+err.Error())
			continue
		} 
		
		// Then try forward the JSON data onwards
		// By first getting the root JSON Key (ChunkType)
		var chunkTypeStringKey string

		// We assume there's only one root key
		for key := range JSONData {
			chunkTypeStringKey = key
			break
		}
			
		// And try tranmit it on the routing threads
		chunkTypeRoutingMap.SendChunkToWebSocket(loggingChannel, chunkTypeStringKey, JSONDataString, router)
	}

	routineCompleteChannel <- true
}

