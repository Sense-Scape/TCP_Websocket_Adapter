package Routines

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
)

func HandleLogging(configJson map[string]interface{}, dataChannel chan map[zerolog.Level]string) {

	// And finally create a logger
	var LogLevel = zerolog.DebugLevel
	var multiWriter = zerolog.MultiLevelWriter(zerolog.Nop())
	var logger = zerolog.New(multiWriter).Level(LogLevel).With().Timestamp().Logger()

	// Now try update the logging level threshold
	if LoggingConfig, exists := configJson["LoggingConfig"].(map[string]interface{}); exists {

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
		var fileName = "Go_TCP_Websocket_Adapter.txt"

		if strings.ToUpper(LoggingConfig["LogToFile"].(string)) == "TRUE" {
			LogToFile = true
		}
		if strings.ToUpper(LoggingConfig["LogToConsole"].(string)) == "TRUE" {
			LogToConsole = true
		}

		// Selectively create log file
		var file *os.File
		var err error
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

	for {
		// Get received JSON data and then
		// Transmit it and then have a nap
		// levelMessageMap := <-dataChannel

	}

}
