package serverconnection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	livestreamconfig "vu/ase/transceiver/src/config"
	"vu/ase/transceiver/src/state"

	rtc "github.com/VU-ASE/roverrtc/src"

	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"
)

type EndpointError struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

func New(serverAddress string, clientId string, processState *state.ProcessState) (*rtc.RTC, error) {
	conn := rtc.NewRTC(clientId)

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(livestreamconfig.PeerConnectionConfig)
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

	// Create the required data channels (meta, control, frame)
	controlChan, err := peerConnection.CreateDataChannel(livestreamconfig.ControlChannelLabel, nil)
	if err != nil {
		return nil, err
	}
	conn.ControlChannel = controlChan
	registerControlDatachannel(controlChan, processState)
	metaChan, err := peerConnection.CreateDataChannel(livestreamconfig.MetaChannelLabel, nil)
	if err != nil {
		return nil, err
	}
	conn.MetaChannel = metaChan
	registerMetaDatachannel(metaChan)
	frameChan, err := peerConnection.CreateDataChannel(livestreamconfig.FrameChannelLabel, nil)
	if err != nil {
		return nil, err
	}
	conn.FrameChannel = frameChan
	registerFrameDatachannel(frameChan)

	// Set handler for connection/disconnection state changes
	peerConnection.OnConnectionStateChange(onConnectionStateChange(conn))

	// Create an offer to send to the other process
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return nil, err
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	// note: this will start the gathering of ICE candidates
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	<-gatherComplete
	log.Info().Msg("ICE gathering complete")

	request := rtc.RequestSDP{
		Offer:     offer,
		Id:        clientId,
		Timestamp: time.Now().UnixMilli(),
	}

	// Send our offer to the signaling server
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(fmt.Sprintf("%s/car/sdp", serverAddress), "application/json; charset=utf-8", bytes.NewReader(payload)) // nolint:noctx
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

	conn.CandidatesLock.Lock()
	defer conn.CandidatesLock.Unlock()

	// Send all the ICE candidates we received to the server
	for _, iceCandidate := range conn.Candidates {
		request := rtc.RequestICE{
			Candidate: iceCandidate,
			Id:        clientId,
		}

		payload, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}
		resp, err := http.Post(fmt.Sprintf("%s/car/ice", serverAddress), "application/json; charset=utf-8", bytes.NewReader(payload)) // nolint:noctx
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
