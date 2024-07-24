package state

import "vu/ase/transceiver/src/publisher"

// This is state that can be easily shared with functions that need it
type ProcessState struct {
	ControllerPublisherQueue publisher.ControllerQueue
}
