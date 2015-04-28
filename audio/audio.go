package audio

import (
	"log"
	"time"

	"github.com/gordonklaus/portaudio"
)

type Iface struct {
	*portaudio.Stream
	Buffer []float32
}

func (i *Iface) Callback(in []float32) {
	for _, r := range in {
		i.Buffer = append(i.Buffer, r)
	}
}

func GetIface(iface int, samplerate float64, buflen int) (*Iface, error) {
	devices, err := portaudio.Devices()
	if err != nil {
		return nil, err
	}
	di := devices[iface]
	streamparams := portaudio.LowLatencyParameters(di, nil)
	streamparams.Input.Channels = 2
	streamparams.FramesPerBuffer = buflen / 2
	var out Iface
	out.Buffer = make([]float32, 0, buflen)
	out.Stream, err = portaudio.OpenStream(streamparams, out.Callback)
	return &out, err
}

func (i Iface) RecordAudio(t time.Duration) (err error) {
	err = i.Start()
	if err != nil {
		return
	}
	<-time.After(t)
	err = i.Stop()
	return
}

func init() {
	if err := portaudio.Initialize(); err != nil {
		log.Panic("couldn't initialize portaudio", err)
	}
}
