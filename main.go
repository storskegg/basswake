// basswake is a blatant copy of the example code provided by the
// Oto codebase as of 2026-09-04. All props to them. Their copyright
// is provided below, and code has been minimally simplified to suit my
// needs.

// Copyright 2019 The Oto Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"io"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

const (
	hz           float64       = 30.0
	duration     time.Duration = 1 * time.Second
	sampleRate   int64         = 48000
	channelCount int           = 2
)

type SineWave struct {
	freq   float64
	length int64
	pos    int64

	channelCount int
	format       oto.Format

	remaining []byte
}

func NewSineWave(freq float64, duration time.Duration, channelCount int, format oto.Format) *SineWave {
	l := int64(channelCount) * 4 * sampleRate * int64(duration) / int64(time.Second)
	l = l / 4 * 4
	return &SineWave{
		freq:         freq,
		length:       l,
		channelCount: channelCount,
		format:       format,
	}
}

func (s *SineWave) Read(buf []byte) (int, error) {
	if len(s.remaining) > 0 {
		n := copy(buf, s.remaining)
		copy(s.remaining, s.remaining[n:])
		s.remaining = s.remaining[:len(s.remaining)-n]
		return n, nil
	}

	if s.pos == s.length {
		return 0, io.EOF
	}

	eof := false
	if s.pos+int64(len(buf)) > s.length {
		buf = buf[:s.length-s.pos]
		eof = true
	}

	var origBuf []byte
	if len(buf)%4 > 0 {
		origBuf = buf
		buf = make([]byte, len(origBuf)+4-len(origBuf)%4)
	}

	length := float64(sampleRate) / float64(s.freq)

	num := 4 * s.channelCount
	p := s.pos / int64(num)
	switch s.format {
	case oto.FormatFloat32LE:
		for i := 0; i < len(buf)/num; i++ {
			bs := math.Float32bits(float32(math.Sin(2*math.Pi*float64(p)/length) * 0.3))
			for ch := 0; ch < channelCount; ch++ {
				buf[num*i+4*ch] = byte(bs)
				buf[num*i+1+4*ch] = byte(bs >> 8)
				buf[num*i+2+4*ch] = byte(bs >> 16)
				buf[num*i+3+4*ch] = byte(bs >> 24)
			}
			p++
		}
	case oto.FormatUnsignedInt8:
		for i := 0; i < len(buf)/num; i++ {
			const max = 127
			b := int(math.Sin(2*math.Pi*float64(p)/length) * 0.3 * max)
			for ch := 0; ch < channelCount; ch++ {
				buf[num*i+ch] = byte(b + 128)
			}
			p++
		}
	case oto.FormatSignedInt16LE:
		for i := 0; i < len(buf)/num; i++ {
			const max = 32767
			b := int16(math.Sin(2*math.Pi*float64(p)/length) * 0.3 * max)
			for ch := 0; ch < channelCount; ch++ {
				buf[num*i+2*ch] = byte(b)
				buf[num*i+1+2*ch] = byte(b >> 8)
			}
			p++
		}
	}

	s.pos += int64(len(buf))

	n := len(buf)
	if origBuf != nil {
		n = copy(origBuf, buf)
		s.remaining = buf[n:]
	}

	if eof {
		return n, io.EOF
	}
	return n, nil
}

func play(context *oto.Context, freq float64, duration time.Duration, channelCount int, format oto.Format) *oto.Player {
	p := context.NewPlayer(NewSineWave(freq, duration, channelCount, format))
	p.Play()
	return p
}

func run() error {
	op := &oto.NewContextOptions{}
	op.SampleRate = int(sampleRate)
	op.ChannelCount = channelCount
	op.Format = oto.FormatFloat32LE

	c, ready, err := oto.NewContext(op)
	if err != nil {
		return err
	}
	<-ready

	var wg sync.WaitGroup
	var players []*oto.Player
	var m sync.Mutex

	wg.Go(func() {
		p := play(c, hz, duration, op.ChannelCount, op.Format)
		m.Lock()
		players = append(players, p)
		m.Unlock()
		time.Sleep(1250 * time.Millisecond)
	})

	wg.Wait()

	// Pin the players not to GC the players.
	runtime.KeepAlive(players)

	return nil
}

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}
