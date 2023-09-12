package main

import (
	"net"
	"os"
	"sync"

	"github.com/Sense-Scape/Go_TCP_Websocket_Adapter/v2/Routines"

	"github.com/rs/zerolog"
)

func main() {

	dataChannel := make(chan string) // Create an integer channel

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	var wg sync.WaitGroup
	wg.Add(1)
	go Routines.HandleConnectionGoWebSocketOutgoing(dataChannel, &wg)

	// Define the port to listen on
	port := "10005"

	// Create a TCP listener on the specified port
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatal().Msg("Error:" + err.Error())
		os.Exit(1)
	}
	defer listener.Close()

	logger.Info().Msg("TCP server is listening on port:" + port)

	// Accept incoming connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error().Msg("Error:" + err.Error())
			continue
		}
		go Routines.HandleConnectionTCPIncomingChunkTypes(dataChannel, conn)
	}

	wg.Wait()
}
