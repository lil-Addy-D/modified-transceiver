package main

import (
	"fmt"
	"os"
	livestreamconfig "vu/ase/transceiver/src/config"
	"vu/ase/transceiver/src/serverconnection"
	"vu/ase/transceiver/src/state"
	"vu/ase/transceiver/src/stream"

	roverlib "github.com/VU-ASE/roverlib-go/src"

	"github.com/rs/zerolog/log"
)

// The actual program
func run(service roverlib.Service, config *roverlib.ServiceConfiguration) error {
	if config == nil {
		return fmt.Errorf("No configuration was provided. Do not know how to proceed")
	}

	// Get server address from service.yaml
	serverAddr, err := config.GetStringSafe("passthrough-address")
	if err != nil {
		return fmt.Errorf("Could not fetch passthrough server address: %v", err)
	}

	// Get the address to output newly received tuning states to
	tuningOutput := service.GetWriteStream("tuning")
	if tuningOutput == nil {
		return fmt.Errorf("Could not fetch tuning output address")
	}

	// Can be accessed by all functions
	state := state.AppState{
		TuningOutputStream: tuningOutput,
	}

	// Initialize connection with the pass-through server
	server, err := serverconnection.New(serverAddr, livestreamconfig.CarId, &state)
	if err != nil {
		return err
	}

	// Start the stream
	errorChan := make(chan error)
	go func() {
		errorChan <- stream.Stream(server, service)
	}()

	// We quit on error
	err = <-errorChan
	if err != nil {
		log.Err(err).Msg("Error while streaming")
		return err
	}
	return nil
}

// Cleanup on termination
func onTerminate(sig os.Signal) error {
	return nil
}

// Start using roverlib
func main() {
	roverlib.Run(run, onTerminate)
}
