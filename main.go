package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
)

var flagLeft = flag.Bool("left", false, "click in left channel")
var flagRight = flag.Bool("right", false, "click in right channel")
var flagBPM = flag.Float64("bpm", 60.0, "bpm")
var flagSampleRate = flag.Float64("sr", 44100.0, "sample rate")
var flagPulseWidth = flag.Float64("pw", 4.8, "pulse width in ms")
var flagVolume = flag.Float64("vol", 1, "volume")

func main() {
	flag.Parse()
	if !*flagLeft && !*flagRight {
		fmt.Println("no channel specified")
		os.Exit(1)
	}
	c, err := New(*flagSampleRate)
	if err != nil {
		panic(err)
	}
	c.Left = *flagLeft
	c.Right = *flagRight
	c.BPM = *flagBPM
	c.PulseWidth = *flagPulseWidth * 1000
	c.Volume = *flagVolume
	c.Start()

	done := make(chan bool)
	ctlc := make(chan os.Signal, 1)
	signal.Notify(ctlc, os.Interrupt)
	go func() {
		for _ = range ctlc {
			done <- true
			c.Stop()
		}
	}()

	<-done
}

type Click struct {
	Volume      float64
	TuneLatency int64
	SampleRate  float64
	BPM         float64
	PulseWidth  float64
	Left, Right bool
	activated   bool
	activate    chan bool
	sampleNum   float64
	done        chan bool
}

func New(sampleRate float64) (c *Click, err error) {
	c = &Click{
		Volume:     1,
		SampleRate: sampleRate,
		BPM:        60,
		Left:       true,
		Right:      true,
		PulseWidth: 4800.0, // microseconds
	}
	c.activate = make(chan bool, 10)
	c.done = make(chan bool)
	sr := beep.SampleRate(int(sampleRate))
	err = speaker.Init(sr, sr.N(time.Second/400))
	return
}

func (c *Click) Start() {
	speaker.Play(c.click())
	ticker := time.NewTicker(time.Duration(60000/c.BPM) * time.Millisecond)
	go func() {
		for {
			select {
			case <-c.done:
				return
			case _ = <-ticker.C:
				if c.TuneLatency > 0 {
					time.Sleep(time.Duration(c.TuneLatency) * time.Millisecond)
				}
				c.activate <- true
			}
		}
	}()
}

func (c *Click) Stop() {
	speaker.Clear()
	speaker.Close()
}

func (c *Click) click() beep.Streamer {
	return beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		for i := range samples {
			select {
			case <-c.activate:
				c.activated = true
				c.sampleNum = 0
			default:
			}
			sample := 0.0
			window := (c.SampleRate * c.PulseWidth / 1000000)
			if c.sampleNum < window && c.activated {
				sample = c.Volume
			}
			if c.Left {
				samples[i][0] = sample
			} else {
				samples[i][0] = 0
			}
			if c.Right {
				samples[i][1] = sample
			} else {
				samples[i][1] = 0
			}
			c.sampleNum++
			if c.sampleNum > c.SampleRate*60/c.BPM {
				c.sampleNum = 0
				c.activated = false
			}
		}
		return len(samples), true
	})
}
