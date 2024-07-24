package publisher

import (
	pb_module_outputs "github.com/VU-ASE/pkg-CommunicationDefinitions/v2/packages/go/outputs"

	zmq "github.com/pebbe/zmq4"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

type ControllerQueue = chan *pb_module_outputs.ControllerOutput

// This publisher transparently replaces the role of the "native" controller publisher.
// This is useful if you want to take control of the car, overriding its normal behavior.
func StartControllerPublisher(address string, controllerQueue ControllerQueue) {
	publisher, _ := zmq.NewSocket(zmq.PUB)
	defer publisher.Close()

	err := publisher.Bind(address)
	if err != nil {
		log.Err(err).Str("address", address).Msg("Error while binding publisher")
		return
	}

	// Main publisher loop
	for {
		// Receive a pointer to the new message
		msg := <-controllerQueue

		// Get the next message
		if msg == nil {
			continue
		}

		// Encode the message as general controller output
		message := pb_module_outputs.SensorOutput{
			SensorId:  1,
			Timestamp: 1,
			SensorOutput: &pb_module_outputs.SensorOutput_ControllerOutput{
				ControllerOutput: msg,
			},
		}

		encodedMsg, err := proto.Marshal(&message)
		if err != nil {
			log.Err(err).Msg("Error while encoding message")
			continue
		}

		// Publish the message
		_, err = publisher.SendBytes(encodedMsg, 0)
		if err != nil {
			log.Err(err).Msg("Error while publishing message")
			continue
		}
	}
}
