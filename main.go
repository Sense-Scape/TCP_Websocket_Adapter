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

	LoggingChannel := make(chan map[zerolog.Level]string)

	routineCount = routineCount + 1
	go Routines.HandleLogging(routineCompleteChannel, serverConfigStringMap, LoggingChannel)

	routineCount = routineCount + 1
	GenericChunkChannel := make(chan string) // Create an integer channel
	go Routines.HandleTCPReceivals(LoggingChannel, GenericChunkChannel)

	for {
		time.Sleep(1 * time.Second)

		myMap := make(map[zerolog.Level]string)
		myMap[zerolog.DebugLevel] = "test"

		LoggingChannel <- myMap

	}

	// for i := 1; i <= routineCount; i++ {
	// 	<-routineCompleteChannel
	// }

	// // Define the TCP port to listen on
	// port := "10100"
	// // Create a channel to pass time chunk json docs around
	// TimeChunkDataChannel := make(chan string) // Create an integer channel

	// // Create a TCP listener on the specified port
	// listener, err := net.Listen("tcp", ":"+port)
	// if err != nil {
	// 	logger.Fatal().Msg("Error:" + err.Error())
	// 	os.Exit(1)
	// }
	// defer listener.Close()
	// logger.Info().Msg("TCP server is listening on port:" + port)

	// // Accept incoming TCP connections
	// for {
	// 	conn, err := listener.Accept()
	// 	if err != nil {
	// 		logger.Error().Msg("Error:" + err.Error())
	// 		continue
	// 	}
	// 	// Start up TCP and websocket threads
	// 	go Routines.HandleTCPReceivals(TimeChunkDataChannel, conn)
	// 	//go Routines.HandleWebSocketTimeChunkTransmissions(TimeChunkDataChannel, "/DataTypes/TimeChunk")
	// }

}
