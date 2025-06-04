package main

import (
	"fmt"
	"io"
	"net"
	"os"

	//"strings"
	"flag"
	"time"
	"vu/ase/transceiver/src/serverconnection"
	"vu/ase/transceiver/src/state"
	"vu/ase/transceiver/src/stream"

	roverlib "github.com/VU-ASE/roverlib-go/src"
	rtc "github.com/VU-ASE/roverrtc/src"
	"github.com/pion/webrtc/v4"

	pb_tuning "github.com/VU-ASE/rovercom/packages/go/tuning"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

// Global value that we can use to clean up on termination
var server *rtc.RTC
var isFuzzing = flag.Bool("fuzz", false, "Enable fuzzing mode")

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
	useWan, err := config.GetFloatSafe("use-wan") // if this is 1, we will use external ICE servers to open a connection
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
	tuningOutput := service.GetWriteStream("transceiver")
	if tuningOutput == nil {
		return fmt.Errorf("Could not fetch transceiver output address")
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

	log.Info().Msg("Starting transceiver with inputs:")
	for _, input := range service.Inputs {
		log.Info().Msgf("  - %s at address", *input.Service)
		for _, stream := range input.Streams {
			log.Info().Msgf("         - %s at %s", *stream.Name, *stream.Address)
		}
	}
	log.Info().Msgf("There are %d inputs in total for this service", len(service.Inputs))

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

	if *isFuzzing {
		log.Info().Msg("Fuzzing mode enabled")

		// SITAS FOR LOOPAS KAD TIPO GAUTUME SOCKET TEISINGA ADRESA, KITAIP SOCKETO NEASSIGNINA ISKART
		for i := 0; i < 10; i++ {
			err := state.TuningOutputStream.WriteBytes([]byte{})
			if err == nil {
				log.Info().Msg("TuningOutputStream is now ready")
				break
			}
			log.Warn().Err(err).Msgf("TuningOutputStream not ready, attempt %d/10", i+1)
			time.Sleep(500 * time.Millisecond)
		}
		// Kai fuzzinsim, reiks gal biski sumazint printu ir pns
		go func() {
			listener, err := net.Listen("tcp", "0.0.0.0:9000")
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to start TCP listener for fuzzing input")
			}
			log.Info().Msg("Listening for fuzzing input on 0.0.0.0:9000")

			for {
				conn, err := listener.Accept()
				if err != nil {
					log.Error().Err(err).Msg("Failed to accept fuzzing connection")
					continue
				}

				go func(c net.Conn) {
					defer c.Close()
					data, err := io.ReadAll(c)
					if err != nil {
						log.Error().Err(err).Msg("Failed to read tuning input")
						return
					}

					tuning := &pb_tuning.TuningState{}
					err = proto.Unmarshal(data, tuning)
					if err != nil {
						log.Error().Err(err).Msg("Failed to unmarshal tuning input")
						return
					}

					log.Info().Str("tuning", tuning.String()).Msg("Received tuning state from harness")
					serverconnection.OnTuningStateReceived(tuning, &state)
				}(conn)
			}
		}()
	}

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
