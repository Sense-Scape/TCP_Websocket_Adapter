package main

import (
	"net"
	"os"

	"github.com/Sense-Scape/Go_TCP_Websocket_Adapter/v2/Routines"

	"github.com/rs/zerolog"
)

func main() {

	// Define the TCP port to listen on
	port := "10005"
	// Create a channel to pass time chunk json docs around
	TimeChunkDataChannel := make(chan string) // Create an integer channel
	// And finally create a logger
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

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
		go Routines.HandleWebSocketTimeChunkTransmissions(TimeChunkDataChannel)
	}

}
