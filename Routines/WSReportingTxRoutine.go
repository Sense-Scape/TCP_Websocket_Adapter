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

	chunkTypeRoutingMap := new(ChunkTypeToChannelMap)

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

	routineCompleteChannel <- true
}

