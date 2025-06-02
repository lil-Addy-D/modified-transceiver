package serverconnection

import (
	"os"
	"vu/ase/transceiver/src/state"

	pb_tuning "github.com/VU-ASE/rovercom/packages/go/tuning"
	rtc "github.com/VU-ASE/roverrtc/src"
	"github.com/pion/webrtc/v4"
	"google.golang.org/protobuf/proto"

	"github.com/rs/zerolog/log"
)

// Clean up and exit if the connection closes
func onConnectionStateChange(conn *rtc.RTC) func(webrtc.PeerConnectionState) {
	log := conn.Log()

	return func(s webrtc.PeerConnectionState) {
		log.Debug().Str("newState", s.String()).Msg("Connection state changed")

		if s == webrtc.PeerConnectionStateClosed || s == webrtc.PeerConnectionStateDisconnected || s == webrtc.PeerConnectionStateFailed {
			log.Warn().Msg("Connection closed, cleaning up")
			conn.Destroy()
			os.Exit(0)
		}
	}
}

// Distribute new tuning parameters to all services, by publishing them to our tuning channel on which the other services are listening
func OnTuningStateReceived(t *pb_tuning.TuningState, appState *state.AppState) {
	log.Info().Str("tuning", t.String()).Msg("Received a new tuning state")

	// Create bytes from the tuning state
	tuning, err := proto.Marshal(t)
	if err != nil {
		log.Err(err).Msg("Could not marshal tuning state")
		return
	}

	// Send the tuning state to the tuning output stream
	err = appState.TuningOutputStream.WriteBytes(tuning)
	if err != nil {
		log.Err(err).Msg("Could not write tuning state to output stream")
	}
}
