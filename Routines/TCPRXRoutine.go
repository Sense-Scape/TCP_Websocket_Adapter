package Routines

import (
	"encoding/binary"
	"net"
	"os"
	"github.com/rs/zerolog"
)

/*
getSessionStates is a partial implementation to extract transmission states of TCP and UDP Headers

returns [transmissionState, sessionNumber, sequenceNumber, transmissionSize]
*/

func HandleTCPReceivals(configJson map[string]interface{}, loggingChannel chan map[zerolog.Level]string, dataChannel chan<- string) {

	// Define the TCP port to listen on
	var port string
	if TCPRxConfig, exists := configJson["TCPRxConfig"].(map[string]interface{}); exists {
		port = TCPRxConfig["Port"].(string)
	} else {
		loggingChannel <- CreateLogMessage(zerolog.FatalLevel, "TCPRx Config not found")
		os.Exit(1)
		return
	}

	// Create a TCP listener on the specified port
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		loggingChannel <- CreateLogMessage(zerolog.FatalLevel, "Error:"+err.Error())
		os.Exit(1)
	}
	defer listener.Close()
	loggingChannel <- CreateLogMessage(zerolog.InfoLevel, "TCP server is listening on port:"+port)

	for {

		conn, err := listener.Accept()
		if err != nil {
			loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "Error:"+err.Error())
		}
		loggingChannel <- CreateLogMessage(zerolog.InfoLevel, "TCP server is connected on port:"+port)

		// Accept incoming TCP connections
		defer conn.Close()

		// Create a buffer to read incoming data

		var byteArray []byte
		var JSONByteArray []byte

		previousSessionNumber := uint32(0)
		previousSequenceNumber := uint32(0)
		sessionContinuous := false
		newSequence := false
		LastInSequence := false

		for {

			// Read data from the connection into the buffer
			buffer := make([]byte, 512)
			bytesRead, err := conn.Read(buffer)
			if bytesRead == 0 {
				loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "Connection from "+conn.RemoteAddr().String()+" closed")
				break
			} else if err != nil {
				loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "Error reading:"+err.Error())
				break
			}

			byteArray = append(byteArray, buffer[:bytesRead]...)
		
			// check if byte array is large enough
			for {

				if len(byteArray) < 512{
					break
				}

				// Expected byte Format
				// |Transport Header(2)| [Session Header(23)|Session Data(x)] |

				// Lets first check how many bytes in the transport layer message
				TransportLayerHeaderSize_bytes := 2
				TransportLayerDataSize := binary.LittleEndian.Uint16(byteArray[:TransportLayerHeaderSize_bytes])
				if TransportLayerDataSize > 512 {
					continue
				}

				//loggingChannel <- CreateLogMessage(zerolog.DebugLevel, "TransportLayerDataSize:"+fmt.Sprint(TransportLayerDataSize))

				// The carry on and extract session state information (v1.0.0 of chunk types)
				SessionLayerHeaderSize_bytes := 23
				transmissionSize := TransportLayerDataSize
				TCPHeaderBytes := byteArray[TransportLayerHeaderSize_bytes : SessionLayerHeaderSize_bytes+TransportLayerHeaderSize_bytes]
				transmissionState, sessionNumber, sequenceNumber := ConvertBytesToSessionStates(TCPHeaderBytes)
				// loggingChannel <- CreateLogMessage(zerolog.DebugLevel, "States: Transmission State "+string(transmissionState)+
				// 	" Session Number "+fmt.Sprint(sessionNumber)+
				// 	" Sequence Number "+fmt.Sprint(sequenceNumber)+
				// 	" Transmission Size "+fmt.Sprint(transmissionSize))

				// Now we check if the Session in continuous
				sessionContinuous, newSequence, LastInSequence, previousSessionNumber, previousSequenceNumber =
					CheckSessionContinuity(transmissionState, sessionNumber, sequenceNumber, previousSessionNumber, previousSequenceNumber)
				// loggingChannel <- CreateLogMessage(zerolog.DebugLevel, "States: sessionContinuous "+fmt.Sprint(sessionContinuous)+
				// 	" newSequence "+fmt.Sprint(newSequence)+
				// 	" LastInSequence "+fmt.Sprint(LastInSequence))

				if newSequence && LastInSequence {
					JSONStartIndex := GetJSONStartIndex()

					JSONByteArray = byteArray[TransportLayerHeaderSize_bytes+SessionLayerHeaderSize_bytes+JSONStartIndex : transmissionSize]
					str := string(JSONByteArray)
					dataChannel <- str

					JSONByteArray = nil
				} else if newSequence && sessionContinuous {
					// Lets start a new receipt sequence
					JSONStartIndex := GetJSONStartIndex()

					JSONByteArray = byteArray[TransportLayerHeaderSize_bytes+SessionLayerHeaderSize_bytes+JSONStartIndex : transmissionSize]

				} else if sessionContinuous && !LastInSequence {
					// Lets keep accumulating data as we have not finished this continuos sequence
					JSONStartIndex := 0
					JSONByteArray = append(JSONByteArray,
						byteArray[TransportLayerHeaderSize_bytes+SessionLayerHeaderSize_bytes+JSONStartIndex:transmissionSize]...)

				} else if sessionContinuous && LastInSequence {
					// We have finished the sequence so we can pass on
					JSONStartIndex := 0
					JSONByteArray = append(JSONByteArray,
						byteArray[TransportLayerHeaderSize_bytes+SessionLayerHeaderSize_bytes+JSONStartIndex:transmissionSize]...)

					str := string(JSONByteArray)
					dataChannel <- str

					JSONByteArray = nil
				} else {
					// There was some error so lets reset
					JSONByteArray = nil

					// The reset all states
					previousSessionNumber = uint32(0)
					previousSequenceNumber = uint32(0)
					sessionContinuous = false
					newSequence = false
					LastInSequence = false

					loggingChannel <- CreateLogMessage(zerolog.WarnLevel, "Missed bytes, resetting")
				}

				byteArray = byteArray[TransportLayerDataSize:]

			}
		}
		loggingChannel <- CreateLogMessage(zerolog.ErrorLevel, "TCP server still listening on port:"+port)
	}

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
		// If it is the first sequence in a session
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
