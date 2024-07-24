package serverconnection

import (
	"vu/ase/transceiver/src/state"

	pb_module_outputs "github.com/VU-ASE/pkg-CommunicationDefinitions/v2/packages/go/outputs"
	pb_systemmanager_messages "github.com/VU-ASE/pkg-CommunicationDefinitions/v2/packages/go/systemmanager"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

// Register all the data channel handlers
func registerMetaDatachannel(dc *webrtc.DataChannel) {
	// Register channel opening handling
	dc.OnOpen(func() {
		log.Info().Str("label", dc.Label()).Msg("Meta datachannel was opened for communication")
	})

	// Register text message handling
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Debug().Str("message", string(msg.Data)).Msg("Received meta message from server")
	})
}

func registerControlDatachannel(dc *webrtc.DataChannel, processState *state.ProcessState) {
	// Register channel opening handling
	dc.OnOpen(func() {
		log.Info().Str("label", dc.Label()).Msg("Control datachannel was opened for communication")
	})

	// Register message handling
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Debug().Msg("Received control message from server")

		// try to parse as sensor output first (hot path)
		remoteSensorData := pb_module_outputs.SensorOutput{}
		err := proto.Unmarshal(msg.Data, &remoteSensorData)
		if err == nil {
			// what kind of sensor output dit we receive?
			switch remoteSensorData.SensorOutput.(type) {
			case *pb_module_outputs.SensorOutput_ControllerOutput:
				log.Debug().Msg("Received human controller data from server")
				// we received controller data
				onControllerData(remoteSensorData.GetControllerOutput(), processState.ControllerPublisherQueue)
				return
			}
		} else {
			log.Warn().Err(err).Msg("Could not parse control message")
			// do not return, try to parse as system manager message
		}

		// the data could also be a tuning state update, try to parse it
		sysmanMsg := pb_systemmanager_messages.SystemManagerMessage{}
		err = proto.Unmarshal(msg.Data, &sysmanMsg)
		if err != nil {
			log.Warn().Err(err).Msg("Could not parse control message")
			return // nothing more to do
		}

		// try to get the tuning state
		tuningState := sysmanMsg.GetTuningState()
		if tuningState != nil {
			log.Debug().Msg("Received tuning state data from server")
			onTuningStateUpdate(tuningState)
		} else {
			log.Warn().Msg("Received unknown control message from server")
		}
	})
}

func registerFrameDatachannel(dc *webrtc.DataChannel) {
	// Register channel opening handling
	dc.OnOpen(func() {
		log.Info().Str("label", dc.Label()).Msg("Frame datachannel was opened for communication")
	})

	// Register message handling
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Debug().Msg("Received frame message from server")
	})
}
