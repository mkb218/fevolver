package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"

	"github.com/mkb218/gosndfile/sndfile"
	"github.com/unixpickle/speechrecog/mfcc"
)

func native_mfcc_euclidean_mono(ref_frames, device_frames []float32, sample_rate int) (score float64) {
	ref_mono := sum_channels_and_normalize(ref_frames)
	device_mono := sum_channels_and_normalize(device_frames)

	var ref_samples, device_samples mfcc.SliceSource
	ref_samples.Slice = ref_mono
	device_samples.Slice = device_mono

	options := &mfcc.Options{}

	ref_mfccs := mfcc.MFCC(&ref_samples, sample_rate, options)
	device_mfccs := mfcc.MFCC(&device_samples, sample_rate, options)

	var sum float64
	var i int
	for {
		log.Printf("round %d", i)
		i++
		ref_coeffs, err := ref_mfccs.NextCoeffs()
		if err != nil {
			log.Println("error in computing reference mfccs: ", err)
			break
		}
		device_coeffs, err := device_mfccs.NextCoeffs()
		if err != nil {
			log.Println("error in computing device mfccs: ", err)
			break
		}

		for i := range ref_coeffs {
			diff := ref_coeffs[i] - device_coeffs[i]
			sum += diff * diff
		}
	}

	return -math.Sqrt(sum)
}

func sum_channels(in []float32) []float64 {
	out := make([]float64, len(in)/2)
	for i, r := range in {
		out[i/2] += float64(r)
	}
	return out
}

func sum_channels_and_normalize(in []float32) []float64 {
	out := sum_channels(in)
	var max float64
	for _, r := range out {
		if math.Abs(r) > max {
			max = math.Abs(r)
		}
	}
	for i := range out {
		out[i] = out[i] / max
	}
	return out
}

func read_frames(source string) (frames []float32, format sndfile.Info, err error) {
	sourcefile, err := sndfile.Open(source, sndfile.Read, &sndfile.Info{})
	if err != nil {
		return nil, sndfile.Info{}, err
	}
	defer func() {
		if err == nil {
			err = sourcefile.Close()
		}
	}()

	if sourcefile.Format.Channels != 2 {
		return nil, sndfile.Info{}, fmt.Errorf("Wrong # of channels in source! 2 != %d", sourcefile.Format.Channels)
	}

	ref_frames := make([]float32, 0)
	buf := make([]float32, 1024)
	for {
		read, err := sourcefile.ReadItems(buf)
		if err != nil {
			return nil, sndfile.Info{}, fmt.Errorf("Couldn't read file: %v", err)
		}
		ref_frames = append(ref_frames, buf[:read]...)
		if read == 0 {
			break
		}
	}
	return ref_frames, sourcefile.Format, nil
}

func main() {
	file_a := flag.String("file_a", "", "first file to compare")
	file_b := flag.String("file_b", "", "second file to compare")
	flag.Parse()
	frames_a, sndinfo, err := read_frames(*file_a)
	if err != nil {
		fmt.Println("couldn't read from file_a: ", err)
		os.Exit(1)
	}
	frames_b, _, err := read_frames(*file_b)
	if err != nil {
		fmt.Println("couldn't read from file_b: ", err)
		os.Exit(1)
	}
	fmt.Println("distance: ", native_mfcc_euclidean_mono(frames_a, frames_b, int(sndinfo.Samplerate)))
}
