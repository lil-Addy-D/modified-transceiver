package main

import (
	"fmt"
	"os"
	livestreamconfig "vu/ase/transceiver/src/config"
	"vu/ase/transceiver/src/publisher"
	"vu/ase/transceiver/src/serverconnection"
	"vu/ase/transceiver/src/state"
	"vu/ase/transceiver/src/stream"

	pb_module_outputs "github.com/VU-ASE/pkg-CommunicationDefinitions/v2/packages/go/outputs"
	pb_systemmanager_messages "github.com/VU-ASE/pkg-CommunicationDefinitions/v2/packages/go/systemmanager"

	"github.com/rs/zerolog/log"

	servicerunner "github.com/VU-ASE/pkg-ServiceRunner/v2/src"
)

// The actual program
func run(service servicerunner.ResolvedService, sysmanInfo servicerunner.SystemManagerInfo, tuningState *pb_systemmanager_messages.TuningState) error {
	// Get server address from service.yaml
	serverAddr, err := servicerunner.GetTuningString("forwardingserver-address", tuningState)
	if err != nil {
		return fmt.Errorf("Could not fetch forwarding server address: %v", err)
	}

	// Create channels for inter-goroutine communication
	controllerPublisherQueue := make(chan *pb_module_outputs.ControllerOutput)
	// Add everything in state, to pass around easily
	state := state.ProcessState{
		ControllerPublisherQueue: controllerPublisherQueue,
	}

	// Start up the server
	server, err := serverconnection.New(serverAddr, livestreamconfig.CarId, &state)
	if err != nil {
		return err
	}

	// Start the stream
	errorChan := make(chan error)
	go func() {
		errorChan <- stream.Stream(server, service, sysmanInfo)
	}()

	// Start up the publisher for the controller output
	outputAddress, err := service.GetOutputAddress("decision")
	if err == nil {
		go publisher.StartControllerPublisher(outputAddress, controllerPublisherQueue)
	} else {
		log.Warn().Err(err).Msg("Controller publisher was not started")
	}

	// We quit on error
	err = <-errorChan
	if err != nil {
		log.Err(err).Msg("Error while streaming")
		return err
	}
	return nil
}

func onTuningState(tuningState *pb_systemmanager_messages.TuningState) {
	// do nothing for now
}

func onTerminate(sig os.Signal) {
	// do nothing for now
}

// Used to start the program with the correct arguments
func main() {
	servicerunner.Run(run, onTuningState, onTerminate, false)
}
