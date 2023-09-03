package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"

	"github.com/rs/zerolog"
)

/*
getSessionStates is a partial implementation to extract transmission states of TCP and UDP Headers

returns [transmissionState, sessionNumber, sequenceNumber, transmissionSize]
*/
func ConvertBytesToSessionStates(byteArray []byte) (byte, uint32, uint32, uint32) {

	fmt.Printf("Byte Array: %v\n", byteArray)

	index := 0

	// Lets first start by extracting the whether this is the finals sequence in session
	transmissionState := byteArray[index]
	index += 1

	// Then we extract session number
	sessionNumber := binary.LittleEndian.Uint32(byteArray[index : index+4])
	index += 4

	// And sequence number
	sequenceNumber := binary.LittleEndian.Uint32(byteArray[index : index+4])
	index += 4

	// We then ignore chunk type
	index += 4
	// And source identifier
	index += 6
	// As there is no support for this at the moment

	// And extract how many bytes were in this session transmission
	transmissionSize := binary.LittleEndian.Uint32(byteArray[index : index+4])

	return transmissionState, sessionNumber, sequenceNumber, transmissionSize
}

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
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	//logger.Info.Msg("Accepted connection from %s\n", conn.RemoteAddr())

	// Create a buffer to read incoming data
	buffer := make([]byte, 512)
	var byteArray []byte

	for {

		// Read data from the connection into the buffer
		bytesRead, err := conn.Read(buffer)
		if err != nil {
			fmt.Println("Error reading:", err)
			break
		}

		byteArray = append(byteArray, buffer[:bytesRead]...)

		// check if byte array is large enough
		if len(byteArray) > 4096 {

			// Expected byte Format
			// |Transport Header(2)| [Session Header(23)|Session Data(x)] |

			TransportLayerHeaderSize := 2
			TransportLayerDataSize := binary.LittleEndian.Uint16(byteArray[:TransportLayerHeaderSize])
			fmt.Printf("States: %d \n", TransportLayerDataSize)

			// Lets start by extracting the TCP byte states
			SessionLayerHeaderSize := 23
			TCPHeaderBytes := byteArray[TransportLayerHeaderSize : SessionLayerHeaderSize+TransportLayerHeaderSize]
			transmissionState, sessionNumber, sequenceNumber, transmissionSize := ConvertBytesToSessionStates(TCPHeaderBytes)
			fmt.Printf("States: %d, %d, %d, %d\n", transmissionState, sessionNumber, sequenceNumber, transmissionSize)

			byteArray = byteArray[TransportLayerDataSize:]
			// FN
			// // Compare previous states to see if

			// // if it remove the non string bytes
			// index := 0

			// // We first extract the base chunk identifier size
			// sourceIdentifierSize := uint16(byteArray[index])<<8 | uint16(byteArray[index+1])
			// fmt.Println("WebSocket server started at", sourceIdentifierSize)

			// // And skip it
			// index += int(sourceIdentifierSize)

			// // We then check the JSON document size
			// JSONDocumentSize := uint16(byteArray[index])<<8 | uint16(byteArray[index+1])
			// fmt.Println("WebSocket server started at", JSONDocumentSize)

			// index += 2

			// // Then extract the string
			// str := string(byteArray[int(index) : int(index)+int(JSONDocumentSize)])
			// fmt.Println(str)
		}
	}

	fmt.Printf("Connection from %s closed\n", conn.RemoteAddr())
}
