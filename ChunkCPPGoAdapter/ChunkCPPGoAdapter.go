// Package phocus_api is the wrapper for the http and queueing systems
package ChunkCPPGoAdapter

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

func HandleConnectionTCPIncomingChunkTypes(dataChannel chan<- string, conn net.Conn) {
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
			transmissionState, sessionNumber, sequenceNumber := ConvertBytesToSessionStates(TCPHeaderBytes)
			logger.Info().Msg("States: Transmission State " + string(transmissionState) +
				" Session Number " + fmt.Sprint(sessionNumber) +
				" Sequence Number " + fmt.Sprint(sequenceNumber) +
				" Transmission Size " + fmt.Sprint(transmissionSize))

			// Now we check if the Session in continuous
			sessionContinuous, newSequence, LastInSequence, previousSessionNumber, previousSequenceNumber =
				CheckSessionContinuity(transmissionState, sessionNumber, sequenceNumber, previousSessionNumber, previousSequenceNumber)
			logger.Info().Msg("States: sessionContinuous " + fmt.Sprint(sessionContinuous) +
				" newSequence " + fmt.Sprint(newSequence) +
				" LastInSequence " + fmt.Sprint(LastInSequence))

			if newSequence {

				JSONStartIndex := GetJSONStartIndex()

				JSONByteArray = byteArray[TransportLayerHeaderSize+SessionLayerHeaderSize+JSONStartIndex : transmissionSize]

			} else if sessionContinuous && !LastInSequence {

				JSONStartIndex := GetJSONStartIndex()

				JSONByteArray = append(JSONByteArray,
					byteArray[TransportLayerHeaderSize+SessionLayerHeaderSize+JSONStartIndex:transmissionSize]...)

			} else if sessionContinuous && LastInSequence {

				JSONStartIndex := GetJSONStartIndex()

				JSONByteArray = append(JSONByteArray,
					byteArray[TransportLayerHeaderSize+SessionLayerHeaderSize+JSONStartIndex:transmissionSize]...)

				str := string(JSONByteArray)
				//logger.Info().Msg(str)
				dataChannel <- str

				JSONByteArray = nil
			} else {
				JSONByteArray = nil
			}

			byteArray = byteArray[TransportLayerDataSize:]

		}
	}

	fmt.Printf("Connection from %s closed\n", conn.RemoteAddr())
}

func ConvertBytesToSessionStates(byteArray []byte) (byte, uint32, uint32) {

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
	return transmissionState, sessionNumber, sequenceNumber
}

/*
CheckSessionContinuity Checks if the session data has arrived in order

returns whether data arrived in order
*/
func CheckSessionContinuity(transmissionState byte, sessionNumber uint32, sequenceNumber uint32, previousSessionNumber uint32, previousSequenceNumber uint32) (bool, bool, bool, uint32, uint32) {

	sessionContinuous := true
	newSequence := false

	// Now we can check all state variables
	// Are we the first message in a sequence?
	StartSequence := (sequenceNumber == 0)
	// If we are, we then want to know if that sequence is continuous
	SequenceContinuous := (sequenceNumber == previousSequenceNumber+1) // intra
	// Now if we know about continuity, is this the last message in the sequence?
	LastInSequence := transmissionState == 1
	// Checking if this message belongs to the same sequences
	SameSession := (sessionNumber == previousSessionNumber) || (sessionNumber == 0) // inter

	// Now we check for continuity
	if StartSequence {
		sessionContinuous = true
		newSequence = true
	} else if StartSequence || (SameSession && !LastInSequence && SequenceContinuous) {
		sessionContinuous = true
		newSequence = false
	} else if LastInSequence && SameSession && SequenceContinuous {
		sessionContinuous = true
		newSequence = false
	} else {
		sessionContinuous = false
		newSequence = true
	}

	// Now update session states
	previousSequenceNumber = sequenceNumber
	previousSessionNumber = sessionNumber

	return sessionContinuous, newSequence, LastInSequence, previousSessionNumber, previousSequenceNumber
}

func GetJSONStartIndex() int {
	return 4
}
