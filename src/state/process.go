package state

import roverlib "github.com/VU-ASE/roverlib-go/src"

// Maintain the state of our current app/process, so that it can be passed around easily
type AppState struct {
	TuningOutputStream *roverlib.ServiceStream
}
