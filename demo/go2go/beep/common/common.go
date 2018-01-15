package common

import (
	"encoding/json"
	"fmt"

	"github.com/keroserene/go-webrtc"
)

// Common is common function for chat
type Common struct {
	pc *webrtc.PeerConnection
}

//
// Preparing SDP messages for signaling.
// generateOffer and generateAnswer are expected to be called within goroutines.
// It is possible to send the serialized offers or answers immediately upon
// creation, followed by subsequent individual ICE candidates.
//
// However, to ease the user's copy & paste experience, in this case we forgo
// the trickle ICE and wait for OnIceComplete to fire, which will contain
// a full SDP mesasge with all ICE candidates, so the user only has to copy
// one message.
//

func (c *Common) generateOffer() {
	fmt.Println("Generating offer...")
	offer, err := c.pc.CreateOffer() // blocking
	if err != nil {
		fmt.Println(err)
		return
	}
	c.pc.SetLocalDescription(offer)
}

func (c *Common) generateAnswer() {
	fmt.Println("Generating answer...")
	answer, err := c.pc.CreateAnswer() // blocking
	if err != nil {
		fmt.Println(err)
		return
	}
	c.pc.SetLocalDescription(answer)
}

func (c *Common) receiveDescription(sdp *webrtc.SessionDescription) {
	err := c.pc.SetRemoteDescription(sdp)
	if nil != err {
		fmt.Println("ERROR", err)
		return
	}
	fmt.Println("SDP " + sdp.Type + " successfully received.")
	if "offer" == sdp.Type {
		go c.generateAnswer()
	}
}

func (c *Common) SignalReceive(msg string) {
	fmt.Printf("receives message: %s\n", msg)

	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(msg), &parsed)
	if nil != err {
		// fmt.Println(err, ", try again.")
		return
	}

	if nil != parsed["sdp"] {
		sdp := webrtc.DeserializeSessionDescription(msg)
		if nil == sdp {
			fmt.Println("Invalid SDP.")
			return
		}
		c.receiveDescription(sdp)
	}

	// Allow individual ICE candidate messages, but this won't be necessary if
	// the remote peer also doesn't use trickle ICE.
	if nil != parsed["candidate"] {
		fmt.Printf("receive candidate message: %v\n", parsed)
		ice := webrtc.DeserializeIceCandidate(msg)
		if nil == ice {
			fmt.Println("Invalid ICE candidate.")
			return
		}
		c.pc.AddIceCandidate(*ice)
		fmt.Println("ICE candidate successfully received.")
	}
}

// Create a PeerConnection.
// If |instigator| is true, create local data channel which causes a
// negotiation-needed, leading to preparing an SDP offer to be sent to the
// remote peer. Otherwise, await an SDP offer from the remote peer, and send an
// answer back.
func (c *Common) Start(instigator bool, signalPeer func(string), producer webrtc.AudioSource, consumerSink webrtc.AudioSink) error {
	fmt.Println("Starting up PeerConnection...")
	// TODO: Try with TURN servers.
	config := webrtc.NewConfiguration(
		webrtc.OptionIceServer("stun:stun.l.google.com:19302"))

	var err error
	c.pc, err = webrtc.NewPeerConnection(config)
	if nil != err {
		fmt.Println("Failed to create PeerConnection.")
		return err
	}

	// Once all ICE candidates are prepared, they need to be sent to the remote
	// peer which will attempt reaching the local peer through NATs.
	c.pc.OnIceComplete = func() {
		fmt.Println("Finished gathering ICE candidates.")
		sdp := c.pc.LocalDescription().Serialize()
		signalPeer(sdp)
	}

	c.pc.OnIceGatheringStateChange = func(state webrtc.IceGatheringState) {
		fmt.Println("Ice Gathering State:", state)
		if webrtc.IceGatheringStateComplete == state {
			// send local description.
		}
	}

	if instigator {
		// OnNegotiationNeeded is triggered when something important has occurred in
		// the state of PeerConnection (such as creating a new data channel), in which
		// case a new SDP offer must be prepared and sent to the remote peer.
		c.pc.OnNegotiationNeeded = func() {
			go c.generateOffer()
		}
		c.pc.AddTrack(webrtc.NewAudioTrack("audio-echo", producer), nil)
	} else {
		c.pc.OnAddTrack = func(r *webrtc.RtpReceiver, s []*webrtc.MediaStream) {
			r.Track().(*webrtc.AudioTrack).AddSink(consumerSink)

			// A much simpler way to echo audio (but less useful
			// for demonstrative purposes) is to just:
			// pc.AddTrack(r.Track(), s)
		}
	}

	return nil
}

func (c *Common) Close() {
	if nil != c.pc {
		c.pc.Close()
	}
}
