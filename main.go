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
	var LogLevel = zerolog.DebugLevel
	var multiWriter = zerolog.MultiLevelWriter(zerolog.Nop())
	var logger = zerolog.New(multiWriter).Level(LogLevel).With().Timestamp().Logger()

	// Open the JSON file for reading
	configFile, err := os.Open("Config.json")
	if err != nil {
		logger.Fatal().Msg("Config file Config.json not found")
		os.Exit(1)
		return
	}
	defer configFile.Close()

	// Create a decoder to read JSON data from the file
	decoder := json.NewDecoder(configFile)

	// Create a variable to store the decoded JSON data
	var serverConfigMap map[string]interface{}
	// Decode the JSON data into a map
	if err := decoder.Decode(&serverConfigMap); err != nil {
		logger.Fatal().Msg("Error decoding JSON:")
		return
	}

	// Now try update the logging level threshold
	if LoggingConfig, exists := serverConfigMap["LoggingConfig"].(map[string]interface{}); exists {

		// Logging level control
		var strLogLevel = LoggingConfig["LoggingLevel"].(string)
		strLogLevel = strings.ToUpper(strLogLevel)

		if strLogLevel == "DEBUG" {
			LogLevel = zerolog.DebugLevel
		} else if strLogLevel == "INFO" {
			LogLevel = zerolog.InfoLevel
		} else if strLogLevel == "WARNING" {
			LogLevel = zerolog.WarnLevel
		} else if strLogLevel == "ERROR" {
			LogLevel = zerolog.ErrorLevel
		} else {
			logger.Fatal().Msg("Error setting log level: " + strLogLevel)
		}

		// Logging output control
		var LogToFile = false
		var LogToConsole = false
		var fileName = "test.txt"

		if strings.ToUpper(LoggingConfig["LogToFile"].(string)) == "TRUE" {
			LogToFile = true
		}
		if strings.ToUpper(LoggingConfig["LogToConsole"].(string)) == "TRUE" {
			LogToConsole = true
		}

		// Selectively create log file
		var file *os.File
		if LogToFile {
			file, err = os.Create(fileName)
			if err != nil {
				logger.Fatal().Msg("Failed to create log file")
			}
			defer file.Close()
		}

		// Create a logger with multiple output writers
		if LogToFile && LogToConsole {
			multiWriter = zerolog.MultiLevelWriter(os.Stdout, file)
		} else if LogToFile && !LogToConsole {
			multiWriter = zerolog.MultiLevelWriter(file)
		} else if !LogToFile && LogToConsole {
			multiWriter = zerolog.MultiLevelWriter(os.Stdout)
		} else {
			logger = zerolog.New(multiWriter).With().Timestamp().Logger()
		}

		logger = zerolog.New(multiWriter).Level(LogLevel).With().Timestamp().Logger()
		logger = logger.Output(multiWriter)

	} else {
		logger.Fatal().Msg("LoggingConfig not found")
		os.Exit(1)
		return
	}

	// Define the TCP port to listen on
	port := "10100"
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
