package main

import (
	"fmt"
	"os"
	"vu/ase/transceiver/src/serverconnection"
	"vu/ase/transceiver/src/state"
	"vu/ase/transceiver/src/stream"

	roverlib "github.com/VU-ASE/roverlib-go/src"
	rtc "github.com/VU-ASE/roverrtc/src"
	"github.com/pion/webrtc/v4"

	"github.com/rs/zerolog/log"
)

// Global value that we can use to clean up on termination
var server *rtc.RTC

// The actual program
func run(service roverlib.Service, config *roverlib.ServiceConfiguration) error {
	if config == nil {
		return fmt.Errorf("No configuration was provided. Do not know how to proceed")
	}

	// Get all configuration from our service.yaml
	serverAddr, err := config.GetStringSafe("passthrough-address") // we are going to connect to this address
	if err != nil {
		return fmt.Errorf("Could not fetch passthrough server address: %v", err)
	}
	connectionIdentifier, err := config.GetStringSafe("connection-identifier") // we are going to identify ourselves with this
	if err != nil {
		return fmt.Errorf("Could not fetch connection identifier: %v", err)
	}
	dataChannelLabel, err := config.GetStringSafe("data-channel-label") // we are going to use this label for our data channel (and the server should use the same)
	if err != nil {
		return fmt.Errorf("Could not fetch data channel label: %v", err)
	}
	controlChannelLabel, err := config.GetStringSafe("control-channel-label") // we are going to use this label for our control channel (and the server should use the same)
	if err != nil {
		return fmt.Errorf("Could not fetch control channel label: %v", err)
	}
	useWan, err := config.GetIntSafe("use-wan") // if this is 1, we will use external ICE servers to open a connection
	if err != nil {
		return fmt.Errorf("Could not fetch use-wan: %v", err)
	}
	peerConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	}
	if useWan != 0 {
		peerConfig = webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{
				{
					URLs: []string{"stun:stun.l.google.com:19302"},
				},
			},
		}
	}

	// Get the address to output newly received tuning states to
	tuningOutput := service.GetWriteStream("tuning")
	if tuningOutput == nil {
		return fmt.Errorf("Could not fetch tuning output address")
	}

	// Can be accessed by all functions
	state := state.AppState{
		TuningOutputStream:   tuningOutput,
		ServerAddress:        serverAddr,
		ConnectionIdentifier: connectionIdentifier,
		DataChannelLabel:     dataChannelLabel,
		ControlChannelLabel:  controlChannelLabel,
		UseWan:               useWan,
		PeerConfig:           peerConfig,
	}

	// Initialize connection with the pass-through server
	server, err = serverconnection.New(&state)
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
	if server != nil {
		log.Info().Msg("Destroying server connection")
		server.Destroy()
	}
	return nil
}

// Start using roverlib
func main() {
	roverlib.Run(run, onTerminate)
}
