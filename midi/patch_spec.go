package midi

import (
	"math/rand"
	"reflect"
)

func init() {
	_ = RandomPatch()
}

type Copyable interface {
	Copy() Copyable
}

type Mutatable interface {
	Mutate(pm float64) Mutatable
}

type ReservedBits byte

func (r ReservedBits) Copy() Copyable {
	return r
}

func (r ReservedBits) Mutate(_ float64) Mutatable {
	return ReservedBits(0)
}

func (r ReservedBits) FieldBytes() []byte {
	return []byte{0}
}

type HasBytes interface {
	FieldBytes() []byte
}

type Int14 int16

func (sr Int14) Copy() Copyable {
	return sr
}

func (sr Int14) Mutate(f float64) Mutatable {
	if r := rand.Float64(); r < f/2 {
		// MIDI
		return Int14(rand.Intn(5))
	} else if r < f {
		return Int14(rand.Intn(5000-100) + 100)
	} else {
		return sr
	}
}

func (sr Int14) FieldBytes() []byte {
	return []byte{byte(sr >> 7), byte(sr & 0x7f)}
}

type FseqHeader struct {
	Name                 string `length:"8"`
	Pad1                 [8]ReservedBits
	StartStepLoopPoint   Int14
	EndStepLoopPoint     Int14
	LoopMode             int8 `min:"0" max:"1"`
	SpeedAdjust          int8
	TempoVelocitySens    int8 `max:"7"`
	FormantPitchMode     int8 `max:"1"`
	FormantNoteAssign    int8
	FormantPitchTuning   int8 `max:"0x7e"`
	FormantSequenceDelay int8 `max:"0x63"`
	FrameDataFormat      int8 `max:"3"`
	Pad2                 [2]ReservedBits
	EndStepValidData     Int14
}

func (f FSEQ) Mutate(pm float64) Mutatable {
	newheader := mutateStruct(reflect.ValueOf(f.FseqHeader), pm).(FseqHeader)
	// log.Println("framedata format", newheader.FrameDataFormat)
	newframes := make([]FseqFrame, int(newheader.FrameDataFormat+1)*128)
	// log.Println("frames", len(newframes))
	for i := range newframes {
		var in reflect.Value
		if len(f.FseqFrames) > 0 {
			in = reflect.ValueOf(f.FseqFrames[i%len(f.FseqFrames)])
		} else {
			in = reflect.ValueOf(FseqFrame{})
		}
		newframes[i] = mutateStruct(in, pm).(FseqFrame)
	}
	return FSEQ{newheader, newframes}
}

type FSEQ struct {
	FseqHeader
	FseqFrames []FseqFrame
}

type FseqFrame struct {
	FundamentalHi, FundamentalLo int8
	VoicedFormantFreqHi          [8]int8
	VoicedFormantFreqLo          [8]int8
	VoicedFormantLvl             [8]int8
	UnvoicedFormantFreqHi        [8]int8
	UnvoicedFormantFreqLo        [8]int8
	UnvoicedFormantFreqLvl       [8]int8
}

type FControlDest struct {
	Dest   int8 `width:"4" max:"0x3"`
	OpType int8 `width:"1" max:"0x1"`
	Op     int8 `width:"3" max:"0x7"`
}

type VoiceCommon struct {
	Name                 string `length:"10"`
	Pad0                 [4]ReservedBits
	Category             int8 `max:"0x16"`
	Pad1                 ReservedBits
	LFO1Waveform         int8 `max:"0x5"`
	LFO1Speed, LFO1Delay int8 `max:"0x63"`
	LFO1KeySync          int8 `max:"0x1"`
	Pad2                 ReservedBits

	LFO1PitchModDepth, LFO1AmpModDepth,
	LFO1FreqModDepth int8 `max:"0x63"`
	LFO2Waveform int8 `max:"0x5"`
	LFO2Speed    int8 `max:"0x63"`
	Pad3         [2]ReservedBits
	LFO2Phase    int8 `max:"0x3"`
	LFO2KeySync  int8 `max:"0x1"`
	NoteShift    int8 `max:"0x30"`
	PitchEGLevel1, PitchEGLevel2,
	PitchEGLevel3, PitchEGLevel4 int8 `max:"0x64"`
	PitchEGTime1, PitchEGTime2,
	PitchEGTime3, PitchEGTime4 int8 `max:"0x64"`
	PitchEGTVeloSensitivity        int8 `max:"0x7"`
	FseqVoicedOpSwitchHi           int8 `max:"0x1"`
	FseqVoicedOpSwitchLo           int8
	FseqUnvoicedOpSwitchHi         int8 `max:"0x1"`
	FseqUnvoicedOpSwitchLo         int8
	AlgoPreset                     int8    `max:"0x57"`
	VoicedOpCarrierLevelCorrection [8]int8 `max:"0xf"`
	Pad4                           [6]ReservedBits
	PitchEGRange                   int8 `max:"0x3"`
	PitchEGTimeScaleDepth          int8 `max:"0x7"`
	VoicedFeedbackLvl              int8 `max:"0x7"`
	PitchEGLvl3                    int8 `max:"0x64"`
	Pad5                           ReservedBits
	FormantControlDestination      [5]FControlDest
	FormantControlDepth            [5]int8
	FMControlDestination           [5]FControlDest
	FMControlDepth                 [5]int8
	FilterType                     int8 `max:"0x5"`
	FilterRez                      int8 `max:"0x74"`
	FilterRezVeloSens              int8 `max:"0xe"`
	FilterCutoffFreq               int8
	FilterEGDepthVelSens           int8
	FilterCutoffFreqLFO1Depth      int8 `max:"0x63"`
	FilterCutoffFreqLFO2Depth      int8 `max:"0x63"`
	FilterCutoffFreqKeyScaleDepth  int8
	FilterCutoffFreqKeyScalePoint  int8
	FilterInputGain                int8 `max:"0x18"`
	Pad6                           [6]ReservedBits
	FilterEGDepth                  int8
	FilterEGLvl4, FilterEGLvl1,
	FilterEGLvl2, FilterEGLvl3 int8 `max:"0x64"`
	FilterEGTime1, FilterEGTime2,
	FilterEGTime3, FilterEGTime4 int8 `max:"0x64"`
	Pad7                           ReservedBits
	FilterEGAttackTimeVelTimeScale int8 `max:"0x3f"`
	Pad8                           ReservedBits
}

type VoicedOp struct {
	OscKeySync                              int8 `width:"2" max:"0x1"`
	OscTranspose                            int8 `width:"6" max:"0x30"`
	OscFreqCoarse                           int8 `max:"0x1f"`
	OscFreqFine                             int8
	OscFreqNoteScaling                      int8    `max:"0x63"`
	OscBwBiasSense                          int8    `width:"5" max:"0xe"`
	OscSpectralForm                         int8    `width:"3" max:"0x7"`
	OscMode                                 int8    `width:"2" max:"0x1"`
	SpectralSkirt                           int8    `width:"3" max:"0x7"`
	FseqTrackNum                            int8    `width:"3" max:"0x7"`
	OscFreqRatioBandSpectrum                int8    `max:"0x63"`
	OscFreqDetune                           int8    `max:"0x1e"`
	OscFreqEGInit, OscFreqEGAttackVal       int8    `max:"0x64"`
	OscFreqEGAttackTime, OscFreqEGDecayTime int8    `max:"0x63"`
	EGLvl, EGTime                           [4]int8 `max:"0x63"`
	EGHoldTime                              int8    `max:"0x63"`
	EGTimeScaling                           int8    `max:"0x7"`
	LvlScalingTotal, LvlScalingBreakPoint,
	LvlScalingLeftDepth, LvlScalingRightDepth int8 `max:"0x63"`
	LvlScalingLeftCurve, LvlScalingRightCurve int8 `max:"0x3"`
	Pad                                       [3]ReservedBits
	FreqBiasSense                             int8 `width:"5" max:"0xe"`
	PitchModSense                             int8 `width:"3" max:"0x7"`
	FreqModSense                              int8 `width:"4" max:"0x7"`
	FreqVeloSense                             int8 `width:"4" max:"0xe"`
	AmpModSense                               int8 `width:"4" max:"0x7"`
	AmpVeloSense                              int8 `width:"4" max:"0xe"`
	EGBiasSense                               int8 `max:"0xe"`
}

type UnvoicedOp struct {
	FormantPitchTranspose                   int8 `max:"0x30"`
	FormantPitchMode                        int8 `width:"3" max:"0x2"`
	FormantPitchCoarse                      int8 `width:"5" max:"0x15"`
	FormantPitchFine                        int8
	FormantPitchNoteScaling                 int8    `max:"0x63"`
	FormantShapeBandwidth                   int8    `max:"0x63"`
	FormantShapeBwBiasSense                 int8    `max:"0xe"`
	FormantReso                             int8    `width:"5" max:"0x7"`
	FormantSkirt                            int8    `width:"3" max:"0x7"`
	OscFreqEGInit, OscFreqEGAttackVal       int8    `max:"0x64"`
	OscFreqEGAttackTime, OscFreqEGDecayTime int8    `max:"0x63"`
	Lvl                                     int8    `max:"0x63"`
	LvlKeyScaling                           int8    `max:"0x0e"`
	EGLvl, EGTime                           [4]int8 `max:"0x63"`
	EGHoldTime                              int8    `max:"0x63"`
	EGTimeScaling                           int8    `max:"0x7"`
	FreqBiasSense                           int8    `max:"0xe"`
	FreqModSense                            int8    `width:"4" max:"0x7"`
	FreqVeloSense                           int8    `width:"4" max:"0xe"`
	AmpModSense                             int8    `width:"4" max:"0x7"`
	AmpVeloSense                            int8    `width:"4" max:"0xe"`
	EGBiasSense                             int8    `max:"0xe"`
}

type Voice struct {
	VoiceCommon
	VoicedParams   [8]VoicedOp
	UnvoicedParams [8]UnvoicedOp
}

type PerfPart struct {
	NoteReserve           int8 `max:"0x20"`
	VoiceBankNumber       int8 `min:"1" max:"1"`
	ProgramNumber         int8
	RcvChannelMax         int8 `min:"0x7f" max:"0x7f"`
	RcvChannel            int8 `min:"0x10" max:"0x10"`
	MonoPoly              int8 `min:"1" max:"1"` // always poly
	MonoPriority          int8 `max:"0x3"`
	FilterSw              int8 `max:"0x1"`
	NoteShift             int8 `max:"0x30"`
	Detune                int8
	VoicedUnvoicedBalance int8
	Volume                int8
	VelocitySenseDepth    int8
	VelocitySenseOffset   int8
	Pan                   int8
	NoteLimitLow          int8 `min:"0" max:"0"`
	NoteLimitHigh         int8 `min:"0x7f" max:"0x7f"`
	DryLevel              int8
	VariationSend         int8
	ReverbSend            int8
	InsertionSwitch       int8 `max:"0x1"`
	LFO1Rate, LFO1PitchModDepth, LFO1Delay, FilterCutoffFreq, FilterResonance,
	EGAttack, EGDecay, EGRelease, Format, FM, FilterEGDepth, PitchEGInit,
	PitchEGAttack, PitchEGREleaseLevel, PitchEGREleaseTime int8
	Portamento                            int8 `max:"0x3"`
	PortamentoTime                        int8
	PitchBendRangeLow, PitchBendRangeHigh int8 `min:"0x10" max:"0x58"`
	PanScaling                            int8 `max:"0x64"`
	PanLFODepth                           int8 `max:"0x63"`
	VeloLimitLow, VeloLimitHigh           int8 `min:"0x1" max:"0x7f"`
	ExpressionLowLimit                    int8
	SustainRcvSw                          int8 `max:"0x1"`
	LFO2Rate, LFO2ModDepth                int8
	Pad                                   [4]ReservedBits
}

type Bitmaps [8][2]int8

func (b Bitmaps) Mutate(pm float64) Mutatable {
	outb := b
	for i := 0; i < 8; i++ {
		for j := 0; j < 2; j++ {
			if rand.Float64() <= pm {
				outb[i][j] = mutateInt8(0, 0x7f)
			}
		}
	}
	return outb
}

type PerfCommon struct {
	Name                          string `length:"12"`
	Pad0                          [2]ReservedBits
	Category                      int8 `max:"0x16"`
	Pad1                          ReservedBits
	PerfVol                       int8            `min:"0x7f"`
	PerfPan                       int8            `min:"0x1" max:"0x7f"`
	PerfNoteShift                 int8            `max:"0x30"`
	Pad2                          [2]ReservedBits // includes individual out
	FseqPart                      int8            `max:"0x4"`
	FseqBank                      int8            `max:"0x0" min:"0x0"` // always 0
	FseqNumber                    ReservedBits    // always 0!
	FseqSpeedRatio                Int14
	FseqStartStepOffset           [2]int8
	FseqStartStepLoopPoint        [2]int8
	FseqEndStepLoopPoint          [2]int8
	FseqLoopMode                  int8 `max:"0x1"`
	FseqPlayMode                  int8 `min:"0x1" max:"0x2"`
	FseqVelocitySensitivity       int8 `max:"0x7"`
	FseqFormatPitchMode           int8 `max:"0x1"`
	FseqKeyOnTrigger              int8 `max:"0x1"`
	Pad3                          ReservedBits
	FseqFormantSequenceDelay      int8 `max:"0x63"`
	FseqLevelVelocitySenstivity   int8
	ControllerPartSwitches        [8]int8 `max:"0xf"`
	ControllerSourceSwitchBitmaps Bitmaps
	ControllerDestinations        [8]int8 `max:"0x2f"`
	ControllerDepths              [8]int8
	ReverbParameters              [24]int8
	VariationParameters           [32]int8
	InsertionParameters           [32]int8
	ReverbType                    int8 `max:"0x10"`
	ReverbPan                     int8 `min:"0x1" max:"0x7f"`
	ReverbReturn                  int8
	VariationType                 int8 `max:"0x1c"`
	VariationPan                  int8 `min:"0x1" max:"0x7f"`
	VariationReturn               int8
	VariationSendReverb           int8
	InsertionType                 int8 `max:"0x1c"`
	InsertionPan                  int8 `min:"0x1" max:"0x7f"`
	InsertionSendReverb           int8
	InsertionSendVariation        int8
	InsertionLevel                int8
	EQLowGain                     int8 `min:"0x34" max:"0x4c"`
	EQLowFreq                     int8 `min:"0x04" max:"0x28"`
	EQLowQ                        int8 `min:"0x01" max:"0x78"`
	EQLowShape                    int8 `max:"0x1"`
	EQMidGain                     int8 `min:"0x34" max:"0x4c"`
	EQMidFreq                     int8 `min:"0xe" max:"0x36"`
	EQMidQ                        int8 `min:"0x01" max:"0x78"`
	EQHighGain                    int8 `min:"0x34" max:"0x4c"`
	EQHighFreq                    int8 `min:"0x1c" max:"0x3a"`
	EQHighQ                       int8 `min:"0x01" max:"0x78"`
	EQHighShape                   int8 `max:"0x1"`
	Pad4                          ReservedBits
	Parts                         [4]PerfPart
}

type Patch struct {
	PerfCommon
	FSEQ
	Voices [4]Voice
}
