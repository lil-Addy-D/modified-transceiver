package serverconnection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"vu/ase/transceiver/src/state"

	rtc "github.com/VU-ASE/roverrtc/src"

	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"
)

type EndpointError struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

func New(state *state.AppState) (*rtc.RTC, error) {
	conn := rtc.NewRTC(state.ConnectionIdentifier)

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(state.PeerConfig)
	if err != nil {
		return nil, err
	}

	// Whenever a new ICE candidate arrives, add it to the connection information, so that we can send it to the server later
	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		conn.CandidatesLock.Lock()
		defer conn.CandidatesLock.Unlock()

		conn.Candidates = append(conn.Candidates, c.ToJSON())
	})

	//
	// Create communication channels
	//

	// - Control channel
	controlChan, err := peerConnection.CreateDataChannel(state.ControlChannelLabel, nil)
	if err != nil {
		return nil, err
	}
	conn.ControlChannel = controlChan
	registerControlChannel(controlChan)
	// - Data channel
	dataChan, err := peerConnection.CreateDataChannel(state.DataChannelLabel, nil)
	if err != nil {
		return nil, err
	}
	conn.DataChannel = dataChan
	registerDataChannel(dataChan, state)

	//
	// Prepare webRTC connection and offer
	//

	// Catch changes in the connection state
	peerConnection.OnConnectionStateChange(onConnectionStateChange(conn))

	// WebRTC offer to the server
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return nil, err
	}

	// Create channel to block until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	// note: this will start the gathering of ICE candidates
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		return nil, err
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	<-gatherComplete
	log.Info().Msg("ICE gathering complete")
	request := rtc.RequestSDP{
		Offer:     offer,
		Id:        state.ConnectionIdentifier,
		Timestamp: time.Now().UnixMilli(),
	}

	// Send our offer
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(fmt.Sprintf("%s/car/sdp", state.ServerAddress), "application/json; charset=utf-8", bytes.NewReader(payload)) // nolint:noctx
	if err != nil {
		return nil, err
	}

	// Convert resp.body to bytes
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Is this an error?
	errResp := EndpointError{}
	if err = json.Unmarshal(body, &errResp); err == nil && errResp.Error {
		return nil, fmt.Errorf("Could not connect. Server returned: %s", errResp.Message)
	}

	// The offer answer is in the response, so parse it
	answer := webrtc.SessionDescription{}
	if err = json.Unmarshal(body, &answer); err != nil {
		return nil, err
	}
	log.Info().Msg("Received and parsed SDP answer from server")

	// Apply the answer as the remote description
	if err = peerConnection.SetRemoteDescription(answer); err != nil {
		panic(err)
	}

	//
	// Send all ICE candidates to the server
	//

	conn.CandidatesLock.Lock()
	defer conn.CandidatesLock.Unlock()

	// Send all the ICE candidates we received
	for _, iceCandidate := range conn.Candidates {
		request := rtc.RequestICE{
			Candidate: iceCandidate,
			Id:        state.ConnectionIdentifier,
		}

		payload, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}
		resp, err := http.Post(fmt.Sprintf("%s/car/ice", state.ServerAddress), "application/json; charset=utf-8", bytes.NewReader(payload)) // nolint:noctx
		if err != nil {
			return nil, err
		}

		// the response is the ICE candidates of the other peer, so parse it
		ice := []webrtc.ICECandidateInit{}
		if err = json.NewDecoder(resp.Body).Decode(&ice); err != nil {
			return nil, err
		}

		// Add each candidate to the peer connection
		for _, candidate := range ice {
			if err = peerConnection.AddICECandidate(candidate); err != nil {
				return nil, err
			}
		}
	}
	log.Info().Msg("Added all ICE candidates to peer connection. Server has received all our ICE candidates")

	return conn, nil
}
