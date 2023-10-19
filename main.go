package main

import (
	"encoding/json"
	"net"
	"os"
	"strings"

	"github.com/Sense-Scape/Go_TCP_Websocket_Adapter/v2/Routines"

	"github.com/rs/zerolog"
)

func main() {

	// And finally create a logger
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	// Open the JSON file for reading
	file, err := os.Open("Config.json")
	if err != nil {
		logger.Fatal().Msg("Config file Config.json not found")
		os.Exit(1)
		return
	}
	defer file.Close()

	// Create a decoder to read JSON data from the file
	decoder := json.NewDecoder(file)

	// Create a variable to store the decoded JSON data
	var serverConfigMap map[string]interface{}
	// Decode the JSON data into a map
	if err := decoder.Decode(&serverConfigMap); err != nil {
		logger.Fatal().Msg("Error decoding JSON:")
		return
	}

	// Now try update the logging level and outputs
	if LoggingConfig, exists := serverConfigMap["LoggingConfig"].(map[string]interface{}); exists {
		var logLevel = LoggingConfig["LoggingLevel"].(string)
		logLevel = strings.ToUpper(logLevel)

		if logLevel == "DEBUG" {
			logger = logger.Level(zerolog.DebugLevel)
		} else if logLevel == "INFO" {
			logger = logger.Level(zerolog.InfoLevel)
		} else if logLevel == "WARNING" {
			logger = logger.Level(zerolog.WarnLevel)
		} else if logLevel == "ERROR" {
			logger = logger.Level(zerolog.ErrorLevel)
		} else {
			logger.Fatal().Msg("Error setting log level: " + logLevel)
		}
	}

	// Define the TCP port to listen on
	port := "10005"
	// Create a channel to pass time chunk json docs around
	TimeChunkDataChannel := make(chan string) // Create an integer channel

	// Create a TCP listener on the specified port
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatal().Msg("Error:" + err.Error())
		os.Exit(1)
	}
	defer listener.Close()
	logger.Info().Msg("TCP server is listening on port:" + port)

	// Accept incoming TCP connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error().Msg("Error:" + err.Error())
			continue
		}
		// Start up TCP and websocket threads
		go Routines.HandleTCPReceivals(TimeChunkDataChannel, conn)
		//go Routines.HandleWebSocketTimeChunkTransmissions(TimeChunkDataChannel, "/DataTypes/TimeChunk")
	}

}
