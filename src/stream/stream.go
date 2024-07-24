package stream

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"vu/ase/transceiver/src/segmentation"

	pb_systemmanager_messages "github.com/VU-ASE/pkg-CommunicationDefinitions/v2/packages/go/systemmanager"
	rtc "github.com/VU-ASE/pkg-Rtc/src"
	servicerunner "github.com/VU-ASE/pkg-ServiceRunner/v2/src"
	zmq "github.com/pebbe/zmq4"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

type StreamState struct {
	// A list of all services as received from the SystemManager
	services []StreamStateService
	// RW lock for the services list
	servicesLock *sync.RWMutex
	// to make sure the goroutine can be stopped
	destroyed bool
}

type StreamStateService struct {
	// The service that is being streamed
	serviceIdentifier *pb_systemmanager_messages.ServiceIdentifier
	// The endpoints that are being streamed
	endpoints []StreamStateServiceEndpoint
}

type StreamStateServiceEndpoint struct {
	// The endpoint that is being streamed
	endpoint *pb_systemmanager_messages.ServiceEndpoint
	// The ZMQ socket that is used to receive data from the endpoint
	socket *zmq.Socket
}

func NewStreamState() *StreamState {
	return &StreamState{
		services:     make([]StreamStateService, 0),
		servicesLock: &sync.RWMutex{},
		destroyed:    false,
	}
}

func NewStreamStateService(service *pb_systemmanager_messages.Service) (*StreamStateService, error) {
	if service == nil {
		return nil, fmt.Errorf("ServiceIdentifier cannot be nil")
	}

	// Create the endpoints
	endpoints := make([]StreamStateServiceEndpoint, 0)
	for _, endpoint := range service.Endpoints {
		if endpoint == nil {
			log.Warn().Msg("Endpoint is nil")
			continue
		}
		endpoint, err := NewStreamStateServiceEndpoint(endpoint)
		if err != nil {
			log.Err(err).Msg("Could not create endpoint")
		} else {
			endpoints = append(endpoints, *endpoint)
		}
	}

	return &StreamStateService{
		serviceIdentifier: service.Identifier,
		endpoints:         endpoints,
	}, nil
}

func NewStreamStateServiceEndpoint(endpoint *pb_systemmanager_messages.ServiceEndpoint) (*StreamStateServiceEndpoint, error) {
	if endpoint == nil {
		return nil, fmt.Errorf("Endpoints cannot be nil")
	}

	// Create ZMQ receiver for imaging data (sub)
	sock, err := zmq.NewSocket(zmq.SUB)
	if err != nil {
		log.Err(err).Msg("Could not create ZMQ socket")
		return nil, fmt.Errorf("Could not create ZMQ socket: %v", err)
	}
	realAddr := strings.ReplaceAll(endpoint.Address, "*", "localhost")

	// Connect to the address in this endpoint
	err = sock.Connect(realAddr)

	log.Debug().Str("address", endpoint.Address).Msg("Connected to service output")
	if err != nil {
		return nil, fmt.Errorf("Could not connect to imaging output: %v", err)
	}
	err = sock.SetSubscribe("")
	if err != nil {
		return nil, fmt.Errorf("Could not subscribe to imaging output: %v", err)
	}

	return &StreamStateServiceEndpoint{
		endpoint: endpoint,
		socket:   sock,
	}, nil
}

func (s *StreamState) Destroy() {
	s.destroyed = true
}

func (s *StreamState) SetServices(newServices *pb_systemmanager_messages.ServiceList) {
	s.servicesLock.Lock()
	defer s.servicesLock.Unlock()

	// Whenever we get a new service list, we want to do the following:
	// Compare the new list with the old list
	// - If an endpoint is added that was not in the old list, create a new socket to listen on
	// - If an endpoint is removed that was in the old list, close the socket gracefully
	// - If and endpoint is in both lists, do nothing

	// Add all new services and endpoints
	for _, newService := range newServices.Services {
		// Check if the service is already in the old list
		var foundService *StreamStateService = nil
		for _, oldService := range s.services {
			if oldService.serviceIdentifier.Name == newService.Identifier.Name && oldService.serviceIdentifier.Pid == newService.Identifier.Pid {
				foundService = &oldService
				break
			}
		}

		// If the service is not in the old list, create a new service
		if foundService == nil {
			newStateService, err := NewStreamStateService(newService)
			if err != nil {
				log.Err(err).Msg("Could not create new service")
				continue
			}
			s.services = append(s.services, *newStateService)
		} else {
			// If the service is in the old list, compare the endpoints
			for _, newEndpoint := range newService.Endpoints {
				// Check if the endpoint is already in the old list
				var foundEndpoint *StreamStateServiceEndpoint = nil
				for _, oldEndpoint := range foundService.endpoints {
					if oldEndpoint.endpoint.Name == newEndpoint.Name && oldEndpoint.endpoint.Address == newEndpoint.Address {
						foundEndpoint = &oldEndpoint
						break
					}
				}

				// If the endpoint is not in the old list, create a new endpoint
				if foundEndpoint == nil {
					newStateEndpoint, err := NewStreamStateServiceEndpoint(newEndpoint)
					if err != nil {
						log.Err(err).Msg("Could not create new endpoint")
						continue
					}
					foundService.endpoints = append(foundService.endpoints, *newStateEndpoint)
				}
			}
		}
	}

	// Remove all old services and endpoints that are not in the new list
	s.services = slices.DeleteFunc(s.services, func(service StreamStateService) bool {
		// is this service in the new list?
		var foundService *StreamStateService = nil
		for _, newService := range newServices.Services {
			if service.serviceIdentifier.Name == newService.Identifier.Name && service.serviceIdentifier.Pid == newService.Identifier.Pid {
				foundService = &service
				break
			}
		}
		if foundService == nil {
			// this service is not in the new list, remove it after closing the sockets
			log.Debug().Str("service", service.serviceIdentifier.Name).Msg("Closing sockets for removed service")
			for _, endpoint := range service.endpoints {
				// close the socket
				endpoint.socket.Close()
			}
			return true
		} else {
			// Delete the endpoints that are not in the new list
			service.endpoints = slices.DeleteFunc(service.endpoints, func(endpoint StreamStateServiceEndpoint) bool {
				// Is this endpoint still in the new service?
				var foundEndpoint *StreamStateServiceEndpoint = nil
				for _, newEndpoint := range foundService.endpoints {
					if endpoint.endpoint.Name == newEndpoint.endpoint.Name && endpoint.endpoint.Address == newEndpoint.endpoint.Address {
						foundEndpoint = &newEndpoint
						break
					}
				}
				if foundEndpoint == nil {
					// this endpoint is not in the new list, close the socket and remove it
					log.Debug().Str("address", endpoint.endpoint.Address).Str("service", foundService.serviceIdentifier.Name).Msg("Closing socket for removed endpoint")
					endpoint.socket.Close()
					return true
				}
				return false
			})
		}
		return false
	})
}

// Concurrency-safe iterate over all services and endpoints
// The caller may assume that all sockets are open and can be used
func (s *StreamState) ForEachEndpoint(
	f func(service StreamStateService, endpoint StreamStateServiceEndpoint),
) {
	s.servicesLock.RLock()
	defer s.servicesLock.RUnlock()

	for _, service := range s.services {
		for _, endpoint := range service.endpoints {
			f(service, endpoint)
		}
	}
}

func Stream(server *rtc.RTC, service servicerunner.ResolvedService, sysmanInfo servicerunner.SystemManagerInfo) error {
	// Create state that can be accessed by the main stream loop and the goroutine responsible for fetching the services
	state := NewStreamState()
	defer state.Destroy()

	// Fetch initial services
	log.Info().Msg("Fetching initial services to stream from")
	services, err := sysmanInfo.GetAllServices()
	if err != nil {
		return fmt.Errorf("Could not get initial services: %v", err)
	}
	log.Info().Msgf("Fetched %d initial services to stream from", len(services.Services))
	state.SetServices(services)

	// Monotonically increasing packet ID
	var packetId int64 = 0

	// Start a goroutine to fetch services and latest tuning state from the SystemManager
	go func() {
		for {
			if state.destroyed {
				return
			}

			log.Debug().Msg("Fetching services to stream from")
			services, err := sysmanInfo.GetAllServices()
			if err != nil {
				log.Err(err).Msg("Error while fetching services")
			} else {
				log.Debug().Msgf("Fetched %d services to stream from", len(services.Services))
				state.SetServices(services)
				_ = atomic.AddInt64(&packetId, 1) // because the packetId is also updated in the main loop
				// Send the new services to the server
				msg := &pb_systemmanager_messages.SystemManagerMessage{
					Msg: &pb_systemmanager_messages.SystemManagerMessage_ServiceList{
						ServiceList: services,
					},
				}

				// Marshal the message
				msgBytes, err := proto.Marshal(msg)
				if err != nil {
					log.Err(err).Msg("Could not marshal service list message")
					continue
				}

				// Segment the message and send it to the server
				err = SegmentAndSend(server, msgBytes, packetId)
				if err != nil {
					log.Err(err).Msg("Could not send service list message to server")
				}
			}

			log.Debug().Msg("Fetching latest tuning state")
			tuning, err := sysmanInfo.GetTuningState()
			if err != nil {
				log.Err(err).Msg("Error while fetching tuning state")
			} else {
				log.Debug().Msgf("Fetched tuning state with %d parameters", len(tuning.DynamicParameters))
				_ = atomic.AddInt64(&packetId, 1) // because the packetId is also updated in the main loop
				// Send the new services to the server
				msg := &pb_systemmanager_messages.SystemManagerMessage{
					Msg: &pb_systemmanager_messages.SystemManagerMessage_TuningState{
						TuningState: tuning,
					},
				}

				// Marshal the message
				msgBytes, err := proto.Marshal(msg)
				if err != nil {
					log.Err(err).Msg("Could not marshal tuning state message")
					continue
				}

				// Segment the message and send it to the server
				err = SegmentAndSend(server, msgBytes, packetId)
				if err != nil {
					log.Err(err).Msg("Could not send tuning state message to server")
				}
			}

			// wait 1 second before fetching again
			time.Sleep(1 * time.Second)
		}
	}()

	// main sending loop
	for {
		_ = atomic.AddInt64(&packetId, 1) // because the packetId is also updated in the goroutine
		state.ForEachEndpoint(func(service StreamStateService, endpoint StreamStateServiceEndpoint) {
			// Non-blocking receive
			endpointMsg, err := endpoint.socket.RecvBytes(zmq.DONTWAIT)
			if err != nil {
				// todo: check for EAGAIN (no message available yet, try again later), so that we can print actual errors
				// log.Err(err).Str("service", service.serviceIdentifier.Name).Str("address", endpoint.endpoint.Address).Msg("Could not receive module output")
				return
			}

			// Wrap the bytes into the debug protobuf message, so that the attached debugger (WebController) knows which service sent the message
			wrappedMsg := &pb_systemmanager_messages.DebugServiceMessage{
				Service:  service.serviceIdentifier,
				Endpoint: endpoint.endpoint,
				Message:  endpointMsg,
				SentAt:   time.Now().UnixMilli(),
			}
			msg, err := proto.Marshal(wrappedMsg)
			if err != nil {
				log.Err(err).Msg("Could not marshal debug message wrapper")
				return
			}

			// Now segment the message and send it to the server
			err = SegmentAndSend(server, msg, packetId)
			if err != nil {
				log.Err(err).Msg("Could not send debug message to server")
			}
		})
	}
}

func SegmentAndSend(server *rtc.RTC, msg []byte, packetId int64) error {
	segments := segmentation.SegmentBuffer(msg, packetId)
	for _, seg := range segments {
		err := server.SendFrameBytes(seg)
		if err != nil {
			return err
		}
	}
	return nil
}
