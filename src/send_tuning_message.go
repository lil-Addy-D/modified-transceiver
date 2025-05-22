package main

import (
	"log"
	"time"

	pb_tuning "github.com/VU-ASE/rovercom/packages/go/tuning"
	"google.golang.org/protobuf/proto"
)

func createTuningMessage() *pb_tuning.TuningState {
	// Create dynamic parameters
	params := []*pb_tuning.TuningState_Parameter{
		{
			Parameter: &pb_tuning.TuningState_Parameter_NumberParameter{
				NumberParameter: &pb_tuning.TuningState_Parameter_NumberParameter{
					Key:   "speed",
					Value: 0.5,
				},
			},
		},
	}

	// Create the tuning state
	tuningState := &pb_tuning.TuningState{
		Timestamp:      uint64(time.Now().UnixNano()), // Use current time as timestamp
		DynamicParameters: params,
	}

	return tuningState
}


// Assuming you have an instance of AppState
var appState *state.AppState

func sendTuningMessage(tuningState *pb_tuning.TuningState) {
	// Marshal the tuning state to bytes
	tuningBytes, err := proto.Marshal(tuningState)
	if err != nil {
		log.Err(err).Msg("Could not marshal tuning state")
		return
	}

	// Send the tuning state to the tuning output stream
	err = appState.TuningOutputStream.WriteBytes(tuningBytes)
	if err != nil {
		log.Err(err).Msg("Could not write tuning state to output stream")
	}
}

func main() {
	// Create a tuning message
	tuningMessage := createTuningMessage()

	// Send the tuning message
	sendTuningMessage(tuningMessage)

	// Keep the application running or handle other logic
	select {}
}

