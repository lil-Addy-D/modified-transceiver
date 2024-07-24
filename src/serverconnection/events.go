package serverconnection

import (
	"os"
	"vu/ase/transceiver/src/publisher"

	pb_module_outputs "github.com/VU-ASE/pkg-CommunicationDefinitions/v2/packages/go/outputs"
	pb_systemmanager_messages "github.com/VU-ASE/pkg-CommunicationDefinitions/v2/packages/go/systemmanager"
	rtc "github.com/VU-ASE/pkg-Rtc/src"
	servicerunner "github.com/VU-ASE/pkg-ServiceRunner/v2/src"
	"github.com/pion/webrtc/v4"

	"github.com/rs/zerolog/log"
)

// Used to debug connection state changes
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

// Fetch the (PS4) controller data from the server and forward it to the actuator directly
func onControllerData(controllerData *pb_module_outputs.ControllerOutput, publisherQueue publisher.ControllerQueue) {
	// now send it off to the actuator by publishing it
	publisherQueue <- controllerData

	log.Debug().Float32("steeringAngle", controllerData.SteeringAngle).Float32("leftThrottle", controllerData.LeftThrottle).Float32("rightThrottle", controllerData.RightThrottle).Bool("frontLights", controllerData.FrontLights).Msg("Sent controller data to actuator")
}

// If a tuning state change came in from the server
func onTuningStateUpdate(tuningState *pb_systemmanager_messages.TuningState) {
	log.Debug().Str("newState", tuningState.String()).Msg("Tuning state changed")

	// Send the tuning state to the system manager, which will broadcast it to all modules
	message := pb_systemmanager_messages.SystemManagerMessage{
		Msg: &pb_systemmanager_messages.SystemManagerMessage_TuningState{
			TuningState: tuningState,
		},
	}
	_, err := servicerunner.SendRequestToSystemManager(&message)
	if err != nil {
		log.Err(err).Msg("Error sending tuning state to system manager")
	}
}
