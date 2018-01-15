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

var username = "Alice"
var wait chan int
var c *common.Common

type Producer struct {
	sync.Mutex
	Sinks []webrtc.AudioSink
}

func (b *Producer) AddAudioSink(s webrtc.AudioSink) {
	b.Lock()
	b.Sinks = append(b.Sinks, s)
	b.Unlock()
}

func (b *Producer) RemoveAudioSink(s webrtc.AudioSink) {
	b.Lock()
	defer b.Unlock()
	for i, s2 := range b.Sinks {
		if s2 == s {
			b.Sinks = append(b.Sinks[:i], b.Sinks[i+1:]...)
		}
	}
}

func run(b *Producer) {
	const (
		sampleRate     = 48000
		chunkRate      = 100
		numberOfFrames = sampleRate / chunkRate
		toneFrequency  = 256
	)
	data := [][]float64{make([]float64, numberOfFrames)}
	count := 0
	x := 0.04
	for next := time.Now(); ; next = next.Add(time.Second / chunkRate) {
		time.Sleep(next.Sub(time.Now()))

		for i := range data[0] {
			if count%(sampleRate/toneFrequency/2) == 0 {
				x = -x
			}
			data[0][i] = x
			count++
		}

		b.Lock()
		for _, sink := range b.Sinks {
			sink.OnAudioData(data, sampleRate)
		}
		b.Unlock()
	}
}

func signalSend(msg string) {
	fmt.Println(msg + "\n")
	for {
		_, err := http.Post("http://localhost:7777/offer", "application/json", bytes.NewBufferString(msg))
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
	http.HandleFunc("/answer", handler)
	http.ListenAndServe(":6666", nil)
}

func main() {
	webrtc.SetLoggingVerbosity(1)

	wait = make(chan int, 1)
	fmt.Println("=== go-webrtc go2go chat demo ===")
	fmt.Println("Welcome, " + username + "!")

	c = &common.Common{}
	b := &Producer{}
	err := c.Start(true, signalSend, b, nil)
	if err != nil {
		fmt.Print(err)
		return
	}

	go run(b)

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
