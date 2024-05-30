package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Sense-Scape/Go_TCP_Websocket_Adapter/v2/Routines"
	"github.com/rs/zerolog"
)

func main() {

	// Create a decoder to read JSON data from the file
	// Open the JSON file for reading
	routineCompleteChannel := make(chan bool)
	var routineCount = 0
	configFile, err := os.Open("Config.json")

	if err != nil {
		fmt.Println("Config not found")
		os.Exit(1)
		return
	}
	defer configFile.Close()

	decoder := json.NewDecoder(configFile)

	// Create a variable to store the decoded JSON data
	var serverConfigStringMap map[string]interface{}
	// Decode the JSON data into a map
	if err := decoder.Decode(&serverConfigStringMap); err != nil {
		fmt.Println("Error reading Config.json")
		return
	}

	routineCount = routineCount + 1
	LoggingChannel := make(chan map[zerolog.Level]string, 1000)

	go Routines.HandleLogging(serverConfigStringMap, routineCompleteChannel, LoggingChannel)

	routineCount = routineCount + 1
	ReportingChannel := make(chan string, 1000)
	go Routines.HandleWSReportingTx(serverConfigStringMap,routineCompleteChannel,LoggingChannel,ReportingChannel)

	routineCount = routineCount + 1
	GenericChunkChannel := make(chan string, 1000)
	go Routines.HandleTCPReceivals(serverConfigStringMap, LoggingChannel, GenericChunkChannel)

	routineCount = routineCount + 1
	go Routines.HandleWSDataChunkTx(serverConfigStringMap, LoggingChannel, GenericChunkChannel, ReportingChannel)

	for {
		time.Sleep(60 * time.Second)

		myMap := make(map[zerolog.Level]string)
		myMap[zerolog.DebugLevel] = "Main keep alive"

		LoggingChannel <- myMap

	}
}
