`fevolver` is a tool for generating Yamaha FS1r patches that are supposed to sound like a piece of reference audio. It is written in Go and uses the following dependencies directly:
 * `github.com/gordonklaus/portaudio` and thus the PortAudio library
 * `github.com/mjibson/go-dsp/fft` 
 * `github.com/mkb218/gosndfile/sndfile` and thus `libsndfile`
 * `github.com/rakyll/portmidi` and thus PortMIDI
 * `github.com/unixpickle/speechrecog/mfcc`