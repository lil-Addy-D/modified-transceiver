package state

import (
	roverlib "github.com/VU-ASE/roverlib-go/src"
	"github.com/pion/webrtc/v4"
)

// Maintain the state of our current app/process, so that it can be passed around easily
type AppState struct {
	TuningOutputStream *roverlib.WriteStream
	// webRTC configuration
	ServerAddress        string
	ConnectionIdentifier string
	DataChannelLabel     string
	ControlChannelLabel  string
	UseWan               int
	PeerConfig           webrtc.Configuration
}
