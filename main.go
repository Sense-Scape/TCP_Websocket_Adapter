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

/*
CheckSessionContinuity Checks if the session data has arrived in order

returns whether data arrived in order
*/
func CheckSessionContinuity(transmissionState byte, sessionNumber uint32, sequenceNumber uint32) (bool, bool, bool) {

	sessionContinuous := true
	newSequence := false

	// Use a closure to store state between function calls
	previousSequenceNumber := uint32(0) // Variable to be captured by the closure
	previousSessionNumber := uint32(0)  // Variable to be captured by the closure

	UpdatePreviousSequenceNumber := func(currentSequenceNumber uint32) {
		previousSequenceNumber = currentSequenceNumber
	}
	UpdatePreviousSessionNumber := func(currentSessionNumber uint32) {
		previousSessionNumber = currentSessionNumber
	}

	// Now we can check all state variables
	// Are we the first message in a sequence?
	bStartSequence := (sequenceNumber == 0)
	// If we are, we then want to know if that sequence is continuous
	bSequenceContinuous := (sequenceNumber == previousSequenceNumber+1) // intra
	// Now if we know about continuity, is this the last message in the sequence?
	bLastInSequence := transmissionState == 1
	// Checking if this message belongs to the same sequences
	SameSession := (sessionNumber == previousSessionNumber) || (sessionNumber == 0) // inter

	// Now we check for continuity
	if bStartSequence {
		sessionContinuous = true
		newSequence = true
	} else if bStartSequence || (SameSession && !bLastInSequence && bSequenceContinuous) {
		sessionContinuous = true
		newSequence = false
	} else if bLastInSequence && SameSession && bSequenceContinuous {
		sessionContinuous = true
		newSequence = false
	} else {
		sessionContinuous = false
		newSequence = true
	}

	// Now update session states
	UpdatePreviousSequenceNumber(sequenceNumber)
	UpdatePreviousSessionNumber(sessionNumber)

	return sessionContinuous, newSequence, bLastInSequence
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

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	// Create a buffer to read incoming data
	buffer := make([]byte, 512)
	var byteArray []byte

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
			fmt.Printf("States: %d \n", TransportLayerDataSize)
			logger.Info().Msg("States:" + fmt.Sprint(TransportLayerDataSize))

			// The carry on and extract session state information (v1.0.0 of chunk types)
			SessionLayerHeaderSize := 23
			TCPHeaderBytes := byteArray[TransportLayerHeaderSize : SessionLayerHeaderSize+TransportLayerHeaderSize]
			transmissionState, sessionNumber, sequenceNumber, transmissionSize := ConvertBytesToSessionStates(TCPHeaderBytes)
			logger.Info().Msg("States: Transmission State" + string(transmissionState) +
				" Session Number " + fmt.Sprint(sessionNumber) +
				" Sequence Number " + fmt.Sprint(sequenceNumber) +
				" Transmission State " + fmt.Sprint(transmissionSize))

			// FN
			// // Compare previous states to see if

			//sessionByteArray := byteArray[TransportLayerHeaderSize+SessionLayerHeaderSize : transmissionSize]
			byteArray = byteArray[TransportLayerDataSize:]

		}
	}

	fmt.Printf("Connection from %s closed\n", conn.RemoteAddr())
}
