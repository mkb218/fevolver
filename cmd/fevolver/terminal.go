package main

import (
	"encoding/gob"
	"fevolver/audio"
	"fevolver/cmd/common"
	"fevolver/midi"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gordonklaus/portaudio"
	"github.com/mjibson/go-dsp/fft"
	"github.com/mkb218/gosndfile/sndfile"
	"github.com/rakyll/portmidi"
	"github.com/unixpickle/speechrecog/mfcc"
)

/*
#cgo LDFLAGS: -laubio
#include <stdlib.h>
#include <aubio/aubio.h>
*/
import "C"

func init() {
	log.SetOutput(os.Stderr)
	rand.Seed(time.Now().UnixNano())
}

func readInt(min, max int) (int, error) {
START:
	var resp string
	_, err := fmt.Scanf("%d", &resp)
	if err != nil {
		return -1, err
	}
	device, err := strconv.ParseInt(resp, 10, 32)
	if err != nil {
		fmt.Println("Your response (", resp, ") was not a number:", err)
		goto START
	}
	d := int(device)
	if d > max || d < min {
		fmt.Println("Your response (", resp, ") was outside bounds ", min, " and ", max)
		goto START
	}
	return d, nil
}

func ListMIDI(headline string) {
	_, err := fmt.Println(headline)
	if err != nil {
		panic(err)
	}
	devs := midi.GetDevices()
	var min int = math.MaxInt16
	var max int
	for i, dev := range devs {
		if dev.IsOutputAvailable {
			if i < min {
				min = i
			}
			if i > max {
				max = i
			}
			fmt.Printf("%d.) %s - %s\n", i, dev.Interface, dev.Name)
		}
	}
	// return readInt(min, max)
}

func ListAudio(headline string) {
	_, err := fmt.Println(headline)
	if err != nil {
		panic(err)
	}
	var min int = math.MaxInt16
	var max int

	devices, err := portaudio.Devices()
	if err != nil {
		log.Println(err)
		return
	}

	for i, device := range devices {
		// if device.MaxInputChannels > 0 {
		if i < min {
			min = i
		}
		if i > max {
			max = i
		}
		_, err := fmt.Printf("%d.) %v - %d channels\n", i, device.Name, device.MaxInputChannels)
		if err != nil {
			panic(err)
		}
		// }
	}
	// return readInt(min, max)
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
	list := flag.Bool("l", false, "list available devices")
	omidi := flag.Int("o", -1, "Output MIDI Device")
	note := flag.Int("mn", 64, "MIDI Note")
	velocity := flag.Int("velo", 127, "MIDI velocity")
	audiodev := flag.Int("a", -1, "Audio device")
	audio_dir := flag.String("tmpdir", "", "(optional) Location for temp audio files")
	statefile := flag.String("s", "state.gob", "(optional) File to hold state")
	popsize := flag.Int("p", 20, "Population size")
	elitism := flag.Int("e", 2, "number of top-ranked individuals to keep unchanged")
	mutation := flag.Float64("m", 0.1, "probability of mutation")
	threshold := flag.Float64("t", 1000, "lower bound for completion")
	max_gen := flag.Int("mg", -1, "maximum number of generations, <0 means only consider threshold")
	source := flag.String("f", "", "audio file source (must be stereo)")
	flag.Parse()
	defer func() {
		err := portaudio.Terminate()
		if err != nil {
			log.Println("terminating portaudio returned an error:", err)
		}
	}()
	if *list {
		ListMIDI("Output MIDI devices:")
		ListAudio("Audio devices:")
		return
	}
	if *omidi == -1 || *audiodev == -1 || *source == "" {
		fmt.Println("-o, -a, and -f are required")
		return
	}
	run_test(*audio_dir, *statefile, *popsize, *elitism, *max_gen, *omidi, *audiodev, *mutation, *threshold,
		*source, int8(*note), int8(*velocity))
}

func run_test(audio_dir string, statefilename string, popsize, elitism, max_gen, midi_dev, audio_dev int,
	mutation, threshold float64, source string, note, velo int8) (sp []common.ScoredPatch, err error) {
	var state common.State
	func() {
		statefile, err := os.Open(statefilename)
		if err != nil {
			fmt.Println("couldn't open statefile!", err)
			return
		}
		defer statefile.Close()
		g := gob.NewDecoder(statefile)
		err = g.Decode(&state)
		if err != nil {
			fmt.Println("couldn't read from statefile!", err)
		}
	}()

	var ref_frames []float32
	var format sndfile.Info
	if state.SourceAudio == nil || len(state.SourceAudio) == 0 {
		ref_frames, format, err = read_frames(source)
		if err != nil {
			fmt.Println(err)
			return
		}
		state.SourceAudio = ref_frames
		state.Format = format
	} else {
		ref_frames = state.SourceAudio
		format = state.Format
	}
	log.Println("Read", len(ref_frames), "samples of source audio")

	var next_gen common.Generation
	if l := len(state.Generations); l > 0 {
		next_gen = state.Generations[l-1]
		new_patches := make([]common.ScoredPatch, 0)
		for _, p := range next_gen.Patches {
			if !filter(p.Score) || p.Filtered {
				log.Println("filter removed", p.Name)
			} else {
				new_patches = append(new_patches, p)
			}
		}
		next_gen.Patches = new_patches
		sort.Sort(&next_gen)
	} else {
		next_gen.Number = -1
	}

GENERATION:
	for (max_gen <= 0) || (next_gen.Number < max_gen) {
		last_gen := next_gen
		next_gen = common.Generation{Number: last_gen.Number + 1}
		log.Println("running test on", next_gen.Number)
		var i int
		for ; i < elitism; i++ {
			if i+1 > len(last_gen.Patches) {
				break
			}
			log.Println("keeping", last_gen.Patches[i].Name, "for elitism")
			next_gen.Patches = append(next_gen.Patches, last_gen.Patches[i])
		}
		dating_pool := last_gen.Patches[i:]
		for i := 0; i < len(dating_pool)-1; i += 2 {
			log.Println("Crossing over", dating_pool[i].Name, "and", dating_pool[i+1].Name)
			child1, child2, err := midi.Crossover(&(dating_pool[i].Patch), &(dating_pool[i+1].Patch))
			if err != nil {
				log.Println("Error crossing over:", err)
				continue
			}

			next_gen.Patches = append(next_gen.Patches, common.ScoredPatch{Patch: *child1}, common.ScoredPatch{Patch: *child2})
		}

		for len(next_gen.Patches) < popsize {
			log.Println("Filling with random patch")
			next_gen.Patches = append(next_gen.Patches, common.ScoredPatch{Patch: midi.RandomPatch()})
		}

		for i := range next_gen.Patches {
			next_gen.Patches[i].Patch = midi.Mutate(next_gen.Patches[i].Patch, mutation)
			next_gen.Patches[i].PerfCommon.Name = fmt.Sprintf("G%dP%d", next_gen.Number, i)
			next_gen.Patches[i].Voices[0].VoiceCommon.Name = fmt.Sprintf("G%dP%dV1", next_gen.Number, i)
			next_gen.Patches[i].Voices[1].VoiceCommon.Name = fmt.Sprintf("G%dP%dV2", next_gen.Number, i)
			next_gen.Patches[i].Voices[2].VoiceCommon.Name = fmt.Sprintf("G%dP%dV3", next_gen.Number, i)
			next_gen.Patches[i].Voices[3].VoiceCommon.Name = fmt.Sprintf("G%dP%dV4", next_gen.Number, i)
			next_gen.Patches[i].FSEQ.Name = fmt.Sprintf("G%dP%d", next_gen.Number, i)
		}

		err := score(next_gen, ref_frames, format, audio_dir, midi_dev, audio_dev, note, velo)
		if err != nil {
			fmt.Println("error scoring:", err)
			return nil, err
		}

		state.Generations = append(state.Generations, next_gen)

		func() {
			var statefile *os.File
			statefile, err = os.Create(statefilename)
			if err != nil {
				fmt.Println("WARNING: couldn't open state file for writing!", err)
				return
			}
			defer statefile.Close()
			g := gob.NewEncoder(statefile)
			err = g.Encode(state)
			if err != nil {
				fmt.Println("WARNING: couldn't save state!", err)
			}
		}()

		for _, p := range next_gen.Patches {
			if p.Score > threshold {
				break GENERATION
			}
		}

		next_patches := make([]common.ScoredPatch, 0, len(next_gen.Patches))
		for _, p := range next_gen.Patches {
			if !filter(p.Score) || p.Filtered {
				log.Println("filter removed", p.Name)
				continue
			}
			next_patches = append(next_patches, p)

		}
		next_gen.Patches = next_patches
		sort.Sort(&next_gen)

	}

	return next_gen.Patches, err
}

func score(gen common.Generation, ref_frames []float32, format sndfile.Info, audio_dir string, midi_dev, audio_dev int, midinote, velocity int8) (err error) {
	midistream, err := midi.OpenStream(portmidi.DeviceID(midi_dev))
	defer midistream.Stream.Close()
	if err != nil {
		return err
	}
	noteon := []portmidi.Event{{Timestamp: 0, Status: 0x90, Data1: int64(midinote & 0x7f), Data2: int64(velocity & 0x7f)}}
	noteoff := []portmidi.Event{{Timestamp: 0, Status: 0x80, Data1: int64(midinote & 0x7f), Data2: int64(velocity & 0x7f)}}
	store_audio := audio_dir != ""
	log.Println("audio", audio_dir, store_audio)

	var ready sync.WaitGroup
	var complete sync.WaitGroup
	var locker = new(sync.RWMutex)
	rectime := time.Duration(len(ref_frames)/2) * time.Second / 44100
	// rectime := time.Duration(4.75 * float64(time.Second))
	for i, p := range gen.Patches {
		if err = midistream.SendPatch(p.Patch); err != nil {
			log.Println("Error sending patch!", err)
			return
		}

		log.Println("sleeping for bulk download")
		time.Sleep(10 * time.Second)

		ready.Add(2)
		complete.Add(2)
		locker.Lock()
		go func() {
			ready.Done()
			locker.RLock()
			defer locker.RUnlock()
			log.Println("noteon")
			err := midistream.Stream.Write(noteon)
			if err != nil {
				log.Panic(err)
			}
			<-time.After(rectime)
			err = midistream.Write(noteoff)
			if err != nil {
				log.Panicln(err)
			}
			log.Println("noteoff")
			complete.Done()
		}()

		var device *audio.Iface
		go func() {
			var err error
			device, err = audio.GetIface(audio_dev, float64(format.Samplerate), 512)
			if err != nil {
				log.Panic(err)
			}
			ready.Done()
			locker.RLock()
			defer locker.RUnlock()
			log.Println("recording")
			err = device.RecordAudio(rectime)
			if err != nil {
				log.Panicln(err)
			}
			complete.Done()
		}()
		ready.Wait()
		locker.Unlock()
		complete.Wait()

		score, filtered := native_mfcc_cosine_mono(ref_frames, device.Buffer, int(format.Samplerate))
		gen.Patches[i].Score = score
		gen.Patches[i].Filtered = filtered
		if audio_dir != "" {
			gen_path := filepath.Join(audio_dir, strconv.FormatInt(int64(gen.Number), 10))
			os.MkdirAll(gen_path, 0755)
			f := format
			f.Format = sndfile.SF_FORMAT_WAV | sndfile.SF_FORMAT_FLOAT
			outfile, err := sndfile.Open(filepath.Join(gen_path, fmt.Sprintf("%d.wav", i)), sndfile.Write, &f)
			if err != nil {
				log.Println("Couldn't open audio file!", err)
				continue
			}
			_, err = outfile.WriteItems(device.Buffer)
			if err != nil {
				log.Println("Couldn't write audio!", err)
			}
		}
		log.Println("gen", gen.Number, "individual", i, "score", score)
	}
	return nil
}

func absolute_difference(ref_frames, devicebuffer []float32) float64 {
	score := float64(0)
	for i := range ref_frames {
		score -= math.Abs(float64(ref_frames[i] - devicebuffer[i]))
	}
	return score
}

func user_rating(ref_frames, devicebuffer []float32) (score float64) {
	for {
		fmt.Println("Enter a score, please (0.0 - 100.0): ")
		_, err := fmt.Scan(&score)
		if err != nil {
			fmt.Println("Bad input:", err)
		}
		if score >= 0 && score <= 100.0 {
			return
		}
	}
}

func aubio_mfcc(ref_frames, devicebuffer []float32, format sndfile.Info) (score float64) {
	// FFT first
	ref_fft_obj := C.new_aubio_fft(C.uint(len(ref_frames)))
	defer C.del_aubio_fft(ref_fft_obj)
	dev_fft_obj := C.new_aubio_fft(C.uint(len(devicebuffer)))
	defer C.del_aubio_fft(dev_fft_obj)

	ref_fft_input := C.new_fvec(C.uint(len(ref_frames)))
	defer C.del_fvec(ref_fft_input)
	ref_fft_output := C.new_cvec(C.uint(len(ref_frames)))
	defer C.del_cvec(ref_fft_output)
	dev_fft_input := C.new_fvec(C.uint(len(devicebuffer)))
	defer C.del_fvec(dev_fft_input)
	dev_fft_output := C.new_cvec(C.uint(len(devicebuffer)))
	defer C.del_cvec(dev_fft_output)

	for i, r := range ref_frames {
		C.fvec_set_sample(ref_fft_input, C.smpl_t(r), C.uint(i))
	}
	for i, r := range devicebuffer {
		C.fvec_set_sample(ref_fft_input, C.smpl_t(r), C.uint(i))
	}

	C.aubio_fft_do(ref_fft_obj, ref_fft_input, ref_fft_output)
	C.aubio_fft_do(dev_fft_obj, dev_fft_input, dev_fft_output)

	var num_mfccs C.uint = 10

	// MFCCs
	ref_mfcc_obj := C.new_aubio_mfcc(C.uint(len(ref_frames)), num_mfccs, num_mfccs*4, C.uint(format.Samplerate))
	defer C.del_aubio_mfcc(ref_mfcc_obj)
	dev_mfcc_obj := C.new_aubio_mfcc(C.uint(len(devicebuffer)), num_mfccs, num_mfccs*4, C.uint(format.Samplerate))
	defer C.del_aubio_mfcc(dev_mfcc_obj)

	ref_mfcc_output := C.new_fvec(num_mfccs)
	defer C.del_fvec(ref_mfcc_output)
	dev_mfcc_output := C.new_fvec(num_mfccs)
	defer C.del_fvec(dev_mfcc_output)

	C.aubio_mfcc_do(ref_mfcc_obj, ref_fft_output, ref_mfcc_output)
	C.aubio_mfcc_do(dev_mfcc_obj, dev_fft_output, dev_mfcc_output)

	// euclidean distance of MFCC
	var sum float32
	for i := C.uint(0); i < num_mfccs; i++ {
		diff := C.fvec_get_sample(ref_mfcc_output, i) - C.fvec_get_sample(dev_mfcc_output, i)
		sum += float32(diff) * float32(diff)
	}
	return -math.Sqrt(float64(sum))
}

var score_frames = user_rating

func passthrough(score float64) bool {
	return true
}

func filter_zeroes(score float64) bool {
	return score > 0
}

var filter = passthrough

func native_fft_euclidean_mono(ref_frames, device_frames []float32) (score float64) {
	ref_mono := sum_channels(ref_frames)
	device_mono := sum_channels(device_frames)
	ref_fft := fft.FFTReal(ref_mono)
	device_fft := fft.FFTReal(device_mono)

	var sum float32
	for i := 0; i < len(ref_fft); i++ {
		// calculate magnitude, don't care about phase
		diff := math.Hypot(real(ref_fft[i]), imag(ref_fft[i])) -
			math.Hypot(real(device_fft[i]), imag(device_fft[i]))
		sum += float32(diff) * float32(diff)
	}
	return -math.Sqrt(float64(sum))
}

func native_mfcc_euclidean_mono(ref_frames, device_frames []float32, sample_rate int) (score float64, filtered bool) {
	ref_mono, ref_max := sum_channels_and_normalize(ref_frames)
	log.Println("ref_max", ref_max)
	device_mono, device_max := sum_channels_and_normalize(device_frames)
	log.Println("device_max", device_max)
	// if device_max < 0.1 {
	// 	log.Println("not enough signal in device frames, filtering")
	// 	return -math.MaxFloat64, true
	// }

	var ref_samples, device_samples mfcc.SliceSource
	ref_samples.Slice = ref_mono
	device_samples.Slice = device_mono

	options := &mfcc.Options{MelCount: 13, LowFreq: 133.33}

	ref_mfccs := mfcc.MFCC(&ref_samples, sample_rate, options)
	device_mfccs := mfcc.MFCC(&device_samples, sample_rate, options)
	// discard first set of coeffs
	ref_mfccs.NextCoeffs()
	device_mfccs.NextCoeffs()
	var sum float64
	for {
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

	return -math.Sqrt(sum), false
}

func native_mfcc_cosine_mono(ref_frames, device_frames []float32, sample_rate int) (score float64, filtered bool) {
	ref_mono := sum_channels(ref_frames)
	device_mono := sum_channels(device_frames)

	var ref_samples, device_samples mfcc.SliceSource
	ref_samples.Slice = ref_mono
	device_samples.Slice = device_mono

	ref_options := &mfcc.Options{MelCount: 13, LowFreq: 133.33, Window: float32(len(ref_mono)) / float32(samplerate) * time.Second}
	device_options := &mfcc.Options{MelCount: 13, LowFreq: 133.33, Window: float32(len(device_mono)) / float32(samplerate) * time.Second}

	ref_mfccs := mfcc.MFCC(&ref_samples, sample_rate, options)
	device_mfccs := mfcc.MFCC(&device_samples, sample_rate, options)
	ref_coeffs := ref_mfccs.NextCoeffs()
	device_coeffs := device_mfccs.NextCoeffs()

	// after https://reference.wolfram.com/language/ref/CosineDistance.html
	var numerator float64
	for i, r := range ref_coeffs {
		numerator = numerator + r*device_coeffs[i]
	}

	var denominator_factors = make([]float64, 2)
	for i := range ref_coeffs {
		denominator_factors[0] = denominator_factors[0] + math.Pow(ref_coeffs[i], 2)
		denominator_factors[1] = denominator_factors[1] + math.Pow(device_coeffs[i], 2)
	}
	var denominator = math.Sqrt(denominator_factors[0]) * math.Sqrt(denominator_factors[1])

	return 1 - (numerator / denominator), false
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
	for i := range out {
		out[i] = out[i] / max
	}
	return out, max
}
