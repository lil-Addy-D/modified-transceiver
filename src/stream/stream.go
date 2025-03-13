package stream

import (
	"time"
	"vu/ase/transceiver/src/segmentation"

	pb_debug "github.com/VU-ASE/rovercom/packages/go/debug"

	roverlib "github.com/VU-ASE/roverlib-go/src"
	rtc "github.com/VU-ASE/roverrtc/src"
	zmq "github.com/pebbe/zmq4"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

// Compact object that can be used to iterate over and quickly access the socket
type inputStream struct {
	Service pb_debug.ServiceIdentifier
	Stream  pb_debug.ServiceEndpoint
	socket  *zmq.Socket
}

// Create a socket if it does not exist (or reopen it if it was closed)
func (s *inputStream) Socket() *zmq.Socket {
	if s.socket != nil {
		return s.socket
	}

	// Create a new socket
	socket, err := zmq.NewSocket(zmq.SUB)
	if err != nil {
		log.Err(err).Msgf("Could not create new socket for service '%s' stream '%s'", s.Service.Name, s.Stream.Name)
		return nil
	}

	// Connect to the endpoint
	err = socket.Connect(s.Stream.Address)
	if err != nil {
		log.Err(err).Msgf("Could not connect to service '%s' stream at address %s '%s'", s.Service.Name, s.Stream.Address, s.Stream.Name)
		return nil
	}

	// Subscribe to all messages
	err = socket.SetSubscribe("")
	if err != nil {
		log.Err(err).Msgf("Could not subscribe to all messages on service '%s' stream '%s'", s.Service.Name, s.Stream.Name)
		return nil
	}

	s.socket = socket
	return socket
}

func Stream(server *rtc.RTC, service roverlib.Service) error {
	streamList := make([]inputStream, 0)

	// Convert all streams to InputStreams so that all streams can be quickly iterated over
	for _, input := range service.Inputs {
		for _, stream := range input.Streams {
			streamList = append(streamList, inputStream{
				Service: pb_debug.ServiceIdentifier{
					Name: *input.Service,
					Pid:  -1,
				},
				Stream: pb_debug.ServiceEndpoint{
					Name:    *stream.Name,
					Address: *stream.Address,
				},
			})
		}
	}

	// Monotonically increasing packet ID
	var packetId int64 = 0

	// Main sending loop: try to read from all dependencies and send the messages fetched to the server
	for {
		packetId++

		// // Send dummy data
		// if packetId%5000 < 10 {

		// 	dummyCore := &pb_outputs.SensorOutput{
		// 		SensorId:  1234,
		// 		Timestamp: uint64(time.Now().UnixMilli()),
		// 		Status:    0,
		// 		SensorOutput: &pb_outputs.SensorOutput_DistanceOutput{
		// 			DistanceOutput: &pb_outputs.DistanceSensorOutput{
		// 				Distance: float32(packetId % 100),
		// 			},
		// 		},
		// 	}

		// 	// Marshal
		// 	dummyData, err := proto.Marshal(dummyCore)
		// 	if err != nil {
		// 		log.Err(err).Msg("Could not marshal dummy data")
		// 		continue
		// 	}

		// 	dummyWrap := &pb_debug.DebugOutput{
		// 		Service:  &pb_debug.ServiceIdentifier{Name: "dummy", Pid: -1},
		// 		Endpoint: &pb_debug.ServiceEndpoint{Name: "dummy", Address: "dummy"},
		// 		Message:  dummyData,
		// 		SentAt:   time.Now().UnixMilli(),
		// 	}

		// 	// Marshal
		// 	dummyMsg, err := proto.Marshal(dummyWrap)
		// 	if err != nil {
		// 		log.Err(err).Msg("Could not marshal dummy message")
		// 		continue
		// 	}

		// 	// Send it off to the server
		// 	err = SegmentAndSendData(server, dummyMsg, packetId)
		// 	if err != nil {
		// 		log.Err(err).Msg("Could not send dummy message to server")
		// 	}

		// 	packetId++
		// }

		for i := range streamList {
			stream := &streamList[i] // need to use index-based access to avoid copying the struct with lock

			log.Debug().Str("service", stream.Service.Name).Str("address", stream.Stream.Address).Msg("Receiving service output")

			// Non-blocking receive
			output, err := stream.Socket().RecvBytes(zmq.DONTWAIT)
			if err != nil {
				log.Debug().Err(err).Str("service", stream.Service.Name).Str("address", stream.Stream.Address).Msg("Could not receive service output")
				// todo: check for EAGAIN (no message available yet, try again later), so that we can print actual errors
				// log.Err(err).Str("service", service.serviceIdentifier.Name).Str("address", endpoint.endpoint.Address).Msg("Could not receive module output")
				continue
			}

			// Wrap it in so that the receiver knows which service the bytes come from
			wrappedMsg := &pb_debug.DebugOutput{
				Service:  &stream.Service,
				Endpoint: &stream.Stream,
				Message:  output,
				SentAt:   time.Now().UnixMilli(),
			}
			msg, err := proto.Marshal(wrappedMsg)
			if err != nil {
				log.Err(err).Msg("Could not marshal debug message wrapper")
				continue
			}

			// Send it off to the server
			err = SegmentAndSendData(server, msg, packetId)
			if err != nil {
				log.Err(err).Msg("Could not send debug message to server")
			}
		}
	}
}

func SegmentAndSendData(server *rtc.RTC, msg []byte, packetId int64) error {
	segments := segmentation.SegmentBuffer(msg, packetId)
	for i, seg := range segments {
		log.Debug().Msgf("Sending segment %d for packet %d", i, packetId)
		err := server.SendDataBytes(seg)
		if err != nil {
			return err
		}
	}
	return nil
}
