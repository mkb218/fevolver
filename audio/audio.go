package audio

import (
	"io"
	"log"
	"math"
	"time"

	"github.com/gordonklaus/portaudio"
	"github.com/shanghuiyang/dtw"
	"github.com/unixpickle/speechrecog/mfcc"
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

const melCount = 13
const lowFreq = 133.33

// https://audiovideotestlab.com/blog/audio-comparison-using-mfcc-and-dtw/
func Native_mfcc_dtw_euclidean_mono(ref_frames, device_frames []float32, sample_rate int) (score float64, filtered bool) {
	ref_mono, ref_max := sum_channels_and_normalize(ref_frames)
	device_mono, device_max := sum_channels_and_normalize(device_frames)

	if ref_max == 0 || device_max == 0 {
		return 0, true
	}

	var ref_samples, device_samples mfcc.SliceSource
	ref_samples.Slice = ref_mono
	device_samples.Slice = device_mono

	ref_options := &mfcc.Options{MelCount: melCount, LowFreq: lowFreq}
	device_options := &mfcc.Options{MelCount: melCount, LowFreq: lowFreq}

	ref_mfcc := mfcc.MFCC(&ref_samples, sample_rate, ref_options)
	device_mfcc := mfcc.MFCC(&device_samples, sample_rate, device_options)
	var ref_coeffs, device_coeffs [][]float64
	for {
		new_ref_coeffs, ref_err := ref_mfcc.NextCoeffs()
		if ref_err != nil {

			if ref_err != io.EOF {
				log.Println("couldn't read from reference source", ref_err)
				return 0, true
			}
			break
		}

		ref_coeffs = append(ref_coeffs, new_ref_coeffs)
	}
	for {
		new_device_coeffs, device_err := device_mfcc.NextCoeffs()
		if device_err != nil {

			if device_err != io.EOF {
				log.Println("couldn't read from reference source", device_err)
				return 0, true
			}
			break
		}

		device_coeffs = append(device_coeffs, new_device_coeffs)
	}
	warper := dtw.New()
	warp_dist, warp_err := warper.Distance(ref_coeffs, device_coeffs, func(x, y interface{}) float64 {
		x_slice, ok := x.([]float64)
		if !ok {
			log.Println("x wasn't a slice of float64!", x)
			return -1
		}
		y_slice, ok := y.([]float64)
		if !ok {
			log.Println("y wasn't a slice of float64!", y)
			return -1
		}

		if len(x_slice) != len(y_slice) {
			log.Println("len(x_slice) != len(y_slice)", len(x_slice), len(y_slice))
			return -1
		}

		var sum float64
		for i := range x_slice {
			diff := x_slice[i] - y_slice[i]
			sum += diff * diff
		}
		x1 := math.Sqrt(sum)
		log.Println(x1)
		return x1
	})

	if warp_err != nil {
		log.Println("dtw error", warp_err)
		return 0, true
	}

	return 1 - warp_dist, false

}

func sum_channels(in []float32) []float64 {
	out := make([]float64, len(in)/2)
	for i, r := range in {
		out[i/2] += float64(r)
	}
	return out
}

func sum_channels_and_normalize(in []float32) ([]float64, float64) {
	out := sum_channels(in)
	var max float64
	for _, r := range out {
		if math.Abs(r) > max {
			max = math.Abs(r)
		}
	}
	if max > 0 {
		for i := range out {
			out[i] = out[i] / max
		}
	}
	return out, max
}
