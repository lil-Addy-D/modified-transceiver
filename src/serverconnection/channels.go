package serverconnection

import (
	"vu/ase/transceiver/src/state"

	pb_tuning "github.com/VU-ASE/rovercom/packages/go/tuning"
	"google.golang.org/protobuf/proto"

	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"
)

//
// Set up all data channels for server communication
// - Data channel, to send debug output to the server and to receive tuning state updates
// - Control channel, for configuration between the server and the transceiver
//

func registerControlChannel(dc *webrtc.DataChannel) {
	// Register channel opening handling
	dc.OnOpen(func() {
		log.Info().Str("label", dc.Label()).Msg("Control channel was opened for communication")
	})

	// Register message handling
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Debug().Msg("Received control message from server")

		//
		// ...
		// message handling code
		// ...
		//

	})
}

func registerDataChannel(dc *webrtc.DataChannel, state *state.AppState) {
	// Register channel opening handling
	dc.OnOpen(func() {
		log.Info().Str("label", dc.Label()).Msg("Data channel was opened for communication")
	})

	// Register message handling
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Debug().Msg("Received data message from server")

		// Try to parse the message as tuning state
		tuning := &pb_tuning.TuningState{}
		err := proto.Unmarshal(msg.Data, tuning)
		if err != nil {
			log.Warn().Err(err).Msg("Could not parse tuning state")
			return
		}

		if tuning.Timestamp != 0 {
			onTuningStateReceived(tuning, state)
		}
	})
}
