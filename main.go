package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"

	ChunkCPPGoAdapter "github.com/Sense-Scape/Go_TCP_Websocket_Adapter/v2/ChunkCPPGoAdapter"
	"github.com/rs/zerolog"
)

func main() {

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

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
		go handleConnectionTCPIncoming(conn)
	}
}

func handleConnectionTCPIncoming(conn net.Conn) {
	defer conn.Close()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	// Create a buffer to read incoming data
	buffer := make([]byte, 512)
	var byteArray []byte
	var JSONByteArray []byte

	previousSessionNumber := uint32(0)
	previousSequenceNumber := uint32(0)
	sessionContinuous := false
	newSequence := false
	LastInSequence := false

	for {

		// Read data from the connection into the buffer
		bytesRead, err := conn.Read(buffer)
		if err != nil {
			logger.Error().Msg("Error reading:" + err.Error())
			break
		}

		byteArray = append(byteArray, buffer[:bytesRead]...)

		// check if byte array is large enough
		if len(byteArray) > 4096 {

			// Expected byte Format
			// |Transport Header(2)| [Session Header(23)|Session Data(x)] |

			// Lets first check how many bytes in the transport layer message
			TransportLayerHeaderSize := 2
			TransportLayerDataSize := binary.LittleEndian.Uint16(byteArray[:TransportLayerHeaderSize])
			logger.Info().Msg("TransportLayerDataSize:" + fmt.Sprint(TransportLayerDataSize))

			// The carry on and extract session state information (v1.0.0 of chunk types)
			SessionLayerHeaderSize := 23
			transmissionSize := TransportLayerDataSize
			TCPHeaderBytes := byteArray[TransportLayerHeaderSize : SessionLayerHeaderSize+TransportLayerHeaderSize]
			transmissionState, sessionNumber, sequenceNumber := ChunkCPPGoAdapter.ConvertBytesToSessionStates(TCPHeaderBytes)
			logger.Info().Msg("States: Transmission State " + string(transmissionState) +
				" Session Number " + fmt.Sprint(sessionNumber) +
				" Sequence Number " + fmt.Sprint(sequenceNumber) +
				" Transmission Size " + fmt.Sprint(transmissionSize))

			// Now we check if the Session in continuous
			sessionContinuous, newSequence, LastInSequence, previousSessionNumber, previousSequenceNumber =
				ChunkCPPGoAdapter.CheckSessionContinuity(transmissionState, sessionNumber, sequenceNumber, previousSessionNumber, previousSequenceNumber)
			logger.Info().Msg("States: sessionContinuous " + fmt.Sprint(sessionContinuous) +
				" newSequence " + fmt.Sprint(newSequence) +
				" LastInSequence " + fmt.Sprint(LastInSequence))

			if newSequence {

				JSONStartIndex := ChunkCPPGoAdapter.GetJSONStartIndex()

				JSONByteArray = byteArray[TransportLayerHeaderSize+SessionLayerHeaderSize+JSONStartIndex : transmissionSize]

			} else if sessionContinuous && !LastInSequence {

				JSONStartIndex := ChunkCPPGoAdapter.GetJSONStartIndex()

				JSONByteArray = append(JSONByteArray,
					byteArray[TransportLayerHeaderSize+SessionLayerHeaderSize+JSONStartIndex:transmissionSize]...)

			} else if sessionContinuous && LastInSequence {

				JSONStartIndex := ChunkCPPGoAdapter.GetJSONStartIndex()

				JSONByteArray = append(JSONByteArray,
					byteArray[TransportLayerHeaderSize+SessionLayerHeaderSize+JSONStartIndex:transmissionSize]...)

				str := string(JSONByteArray)
				logger.Info().Msg(str)

				JSONByteArray = nil
			} else {
				JSONByteArray = nil
			}

			byteArray = byteArray[TransportLayerDataSize:]

		}
	}

	fmt.Printf("Connection from %s closed\n", conn.RemoteAddr())
}

func handleConnectionWebSocketOutgoing() {

}
