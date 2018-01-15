/*
 * Webrtc chat demo.
 * Send chat messages via webrtc, over go.
 * Can interop with the JS client. (Open chat.html in a browser)
 *
 * To use: `go run chat.go`
 */
package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"bytes"
	"io/ioutil"
	"sync"
	"time"

	"github.com/keroserene/go-webrtc"
	"github.com/keroserene/go-webrtc/demo/go2go/beep/common"
)

var username = "Bob"
var wait chan int
var c *common.Common

type Consumer struct {
	sync.Mutex
	Sinks []webrtc.AudioSink
}

func (b *Consumer) AddAudioSink(s webrtc.AudioSink) {
	b.Lock()
	b.Sinks = append(b.Sinks, s)
	b.Unlock()
}

func (b *Consumer) RemoveAudioSink(s webrtc.AudioSink) {
	b.Lock()
	defer b.Unlock()
	for i, s2 := range b.Sinks {
		if s2 == s {
			b.Sinks = append(b.Sinks[:i], b.Sinks[i+1:]...)
		}
	}
}

func (b *Consumer) OnAudioData(data [][]float64, sampleRate float64) {
	fmt.Println("start receving audio data")
}

func signalSend(msg string) {
	fmt.Println(msg + "\n")
	for {
		_, err := http.Post("http://localhost:6666/answer", "application/json", bytes.NewBufferString(msg))
		if err != nil {
			time.Sleep(time.Second * 5)
		} else {
			return
		}
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	buffer, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("read answer body error")
	}
	c.SignalReceive(string(buffer))
}

func startWebServer() {
	http.HandleFunc("/offer", handler)
	http.ListenAndServe(":7777", nil)
}

func main() {
	webrtc.SetLoggingVerbosity(1)

	wait = make(chan int, 1)
	fmt.Println("=== go-webrtc go2go chat demo ===")
	fmt.Println("Welcome, " + username + "!")

	// start as the "answerer."
	c = &common.Common{}
	b := &Consumer{}
	err := c.Start(false, signalSend, nil, b)
	if err != nil {
		fmt.Print(err)
		return
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	go func() {
		<-sigs
		fmt.Println("Demo interrupted. Disconnecting...")
		c.Close()
		os.Exit(1)
	}()

	go startWebServer()

	<-wait
	fmt.Println("done")
}
