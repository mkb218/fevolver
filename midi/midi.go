package midi

import (
	"log"
	"math/rand"
	"reflect"
	"strconv"

	"github.com/rakyll/portmidi"
)

type Stream struct {
	*portmidi.Stream
}

var mutatorInt = reflect.TypeOf(new(Mutatable)).Elem()

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	err := portmidi.Initialize()
	if err != nil {
		log.Println("error initializing portmidi", err)
	}
	log.Println("portmidi initialized")
}

const (
	SimpleFseqCrossover = iota
	FseqSwap
)

const HeaderLen = 9
const FooterLen = 2
const PerfCommonLen = 400
const VoiceParamLen = 608
const FrameDataFormatOffset = 0x1b
const FseqPartOffset = 0x15
const FseqCrossoverPoint = PerfCommonLen + (VoiceParamLen * 4) + FrameDataFormatOffset

func stripEnvelope(s []byte) []byte {
	// log.Printf("len before stripping %d %x", len(s), s[:HeaderLen])
	b := s[HeaderLen:]
	b = b[:len(b)-FooterLen]
	// log.Println("len after stripping", len(b))
	return b
}

func addEnvelopes(in []byte) (out []byte) {

	out = append(out, envelope(perfcommonaddr, in[:PerfCommonLen])...)
	in = in[PerfCommonLen:]
	out = append(out, envelope(voice1addr, in[:VoiceParamLen])...)
	in = in[VoiceParamLen:]
	out = append(out, envelope(voice2addr, in[:VoiceParamLen])...)
	in = in[VoiceParamLen:]
	out = append(out, envelope(voice3addr, in[:VoiceParamLen])...)
	in = in[VoiceParamLen:]
	out = append(out, envelope(voice4addr, in[:VoiceParamLen])...)
	in = in[VoiceParamLen:]
	if len(in) > 0 {
		out = append(out, envelope(fseqaddr, in)...)
	}
	// log.Println("total out with envelopes", len(out))
	return
}

func Crossover(p, p1 *Patch) (child1, child2 *Patch, err error) {
	msgs1 := p.Msgs()
	msgs2 := p1.Msgs()

	log.Println("part count", len(msgs1), len(msgs2))
	log.Println("fseq data", len(p.FSEQ.FseqFrames), len(p1.FSEQ.FseqFrames))

	// crossover
	var bytes1, bytes2 []byte
	for _, m := range msgs1 {
		// log.Println("len msg 1", len(m))
		bytes1 = append(bytes1, stripEnvelope(m)...)
	}
	for _, m := range msgs2 {
		bytes2 = append(bytes2, stripEnvelope(m)...)
	}
	crossovermax := len(bytes1)
	biggest := crossovermax
	if l := len(bytes2); l < crossovermax {
		// log.Println("bytes2 is shorter")
		crossovermax = l
	} else {
		biggest = l
	}

	crossover := rand.Intn(crossovermax)
	// log.Println("crossover point", crossover, crossovermax, biggest)
	var newbytes1 = make([]byte, 0, biggest)
	var newbytes2 = make([]byte, 0, biggest)
	for i := 0; i < biggest; i++ {
		// log.Println("orig", len(bytes1), len(bytes2), "new", len(newbytes1), len(newbytes2))
		if i < crossover {
			if len(bytes1) > i {
				newbytes1 = append(newbytes1, bytes1[i])
			}
			if len(bytes2) > i {
				newbytes2 = append(newbytes2, bytes2[i])
			}
		} else {
			if len(bytes2) > i {
				newbytes1 = append(newbytes1, bytes2[i])
			}
			if len(bytes1) > i {
				newbytes2 = append(newbytes2, bytes1[i])
			}
		}
	}
	if len(bytes1) != len(bytes2) {
		// log.Println("len1", len(bytes1), "len2", len(bytes2), "ack", FseqCrossoverPoint)
		newbytes1 = adjustInternals(len(msgs2), newbytes1)
		newbytes2 = adjustInternals(len(msgs1), newbytes2)
	}

	child1, err = FromByteArray(addEnvelopes(newbytes1))
	if err != nil {
		log.Println("child 1 error: ", err)
		return
	}

	child2, err = FromByteArray(addEnvelopes(newbytes2))
	if err != nil {
		log.Println("child 2 error: ", err)
	}

	return
}

func adjustInternals(numParts int, buf []byte) []byte {
	// log.Println("parts", numParts, "buflen", len(buf))
	if numParts < 5 {
		log.Panicln("not enough parts for a full perf!")
	}
	if numParts == 5 && len(buf) != PerfCommonLen+(VoiceParamLen*4) {
		log.Println("WARNING, parts said", numParts, "(not enough for fseq data) but len was long!", len(buf))
	}
	if numParts == 5 || len(buf) == PerfCommonLen+(VoiceParamLen*4) || buf[FseqPartOffset] == 0 {
		// no fseq data, make sure fseq part is 0
		// log.Println("trimming buf")
		buf[FseqPartOffset] = 0
		buf = buf[:PerfCommonLen+(VoiceParamLen*4)]
		numParts = 5
	}
	if numParts == 6 {
		if buf[FseqPartOffset] != 0 {
			buf[FseqPartOffset] = byte(rand.Intn(5))
			if len(buf) < PerfCommonLen+(VoiceParamLen*4) {
				log.Panicln("numParts was", numParts, "but there wasn't enough data in buf")
			}
		}
		// correct the frame data format
		fseqdatasize := len(buf) - (PerfCommonLen + (VoiceParamLen * 4) + fseqheaderlen)
		// log.Println("total fseq bytes", fseqdatasize, len(buf), "- (", PerfCommonLen, "+ (", VoiceParamLen, "* 4) +", fseqheaderlen, ")")
		fseqdatasize /= 0x32
		// log.Println("total fseq frames", fseqdatasize)
		fseqdatasize /= 128
		fseqdatasize -= 1
		// log.Println("frame data format", fseqdatasize)
		buf[FseqCrossoverPoint] = byte(fseqdatasize)
	}
	return buf
}

func Mutate(p Patch, pm float64) (out Patch) {
	out.PerfCommon = mutateStruct(reflect.ValueOf(p.PerfCommon), pm).(PerfCommon)
	for i := range out.Parts {
		out.Parts[i].ProgramNumber = int8(i)
	}
	for i := range out.Voices {
		out.Voices[i] = mutateStruct(reflect.ValueOf(p.Voices[i]), pm).(Voice)
	}
	if out.PerfCommon.FseqPart != 0 {
		out.FSEQ = p.FSEQ.Mutate(pm).(FSEQ)
	}
	// fix up internals
	return
}

func parseField(fieldtype reflect.StructField, tagname string, defaultVal int8) int8 {
	if str := fieldtype.Tag.Get(tagname); str != "" {
		n, err := strconv.ParseInt(str, 0, 8)
		if err != nil {
			panic(err)
		}
		return int8(n)
	}
	return defaultVal
}

func mutateInt8(min, max int8) int8 {
	if min == max {
		return min
	}
	return int8(rand.Int63n(int64(max-min)) + int64(min))
}

func mutateStruct(rv reflect.Value, pm float64) interface{} {
	t := rv.Type()
	if t.Kind() != reflect.Struct {
		log.Panic("only should ever be called on structs or Mutatables")
	}
	out := reflect.New(rv.Type())
	var i int
	defer func() {
		if r := recover(); r != nil {
			log.Println(i, rv, rv.Type().Name())
			if rv.Kind() == reflect.Struct {
				log.Println(rv.Type().Field(i).Name)
			}
			log.Panic(r)
		}
	}()
	for i = 0; i < rv.Type().NumField(); i++ {
		fieldval := rv.Field(i)
		fieldtype := rv.Type().Field(i)
		if m, ok := fieldval.Interface().(Mutatable); ok {
			out.Elem().Field(i).Set(reflect.ValueOf(m.Mutate(pm)))
			continue
		}
		switch fieldtype.Type.Kind() {
		case reflect.Int8:
			if choice := rand.Float64(); choice > pm {
				out.Elem().Field(i).Set(rv.Field(i))
			} else {
				var min = parseField(fieldtype, "min", 0)
				var max int8 = parseField(fieldtype, "max", 0x7f)
				out.Elem().Field(i).SetInt(int64(mutateInt8(min, max)))
			}
		case reflect.Array:
			var min = parseField(fieldtype, "min", 0)
			var max int8 = parseField(fieldtype, "max", 0x7f)
			elem := reflect.New(fieldtype.Type)
			for e := 0; e < fieldval.Len(); e++ {
				if m, ok := fieldval.Index(e).Interface().(Mutatable); ok {
					elem.Elem().Index(e).Set(reflect.ValueOf(m.Mutate(pm)))
					continue
				}
				switch fieldtype.Type.Elem().Kind() {
				case reflect.Int8:
					elem.Elem().Index(e).SetInt(int64(mutateInt8(min, max)))
				case reflect.Struct:
					elem.Elem().Index(e).Set(reflect.ValueOf(mutateStruct(fieldval.Index(e), pm)))
				default:
					log.Panicln("couldn't mutate", fieldtype)
				}
			}
			out.Elem().Field(i).Set(elem.Elem())
		case reflect.String:
			out.Elem().Field(i).SetString("")
		case reflect.Struct:
			out.Elem().Field(i).Set(reflect.ValueOf(mutateStruct(fieldval, pm)))
		default:
			log.Panicln("Couldn't figure out how to mutate", fieldtype)
		}
	}

	return out.Elem().Interface()
}

func RandomPatch() Patch {
	return Mutate(Patch{}, 1)
}

const (
	perfcommonaddr = 0x100000
	voice1addr     = 0x400000
	voice2addr     = 0x410000
	voice3addr     = 0x420000
	voice4addr     = 0x430000
	fseqaddr       = 0x600000
)

func envelope(addr int, in []byte) []byte {
	out := []byte{0xf0, 0x43, 0, 0x5e,
		byte(len(in)>>7) & 0x7f, byte(len(in) & 0x7f),
		byte(addr >> 16), byte(addr >> 8), byte(addr)}
	out = append(out, in...)
	ck := (int8(checksum(out[4:])) * -1) & 0x7f
	out = append(out, byte(ck), 0xf7)
	return out
}

// return sysex messages
func (p Patch) Msgs() [][]byte {
	var out [][]byte
	b := reflectBytes(reflect.ValueOf(p.PerfCommon))
	// log.Printf("reflectBytes results length %d %x", len(b), b[:HeaderLen])
	out = append(out, envelope(perfcommonaddr, b))
	out = append(out, envelope(voice1addr, reflectBytes(reflect.ValueOf(p.Voices[0]))))
	out = append(out, envelope(voice2addr, reflectBytes(reflect.ValueOf(p.Voices[1]))))
	out = append(out, envelope(voice3addr, reflectBytes(reflect.ValueOf(p.Voices[2]))))
	out = append(out, envelope(voice4addr, reflectBytes(reflect.ValueOf(p.Voices[3]))))
	if p.FseqPart != 0 {
		out = append(out, envelope(fseqaddr, reflectBytes(reflect.ValueOf(p.FSEQ))))
	}
	return out

}

func reflectBytes(pr reflect.Value) (out []byte) {
	if hb, ok := pr.Interface().(HasBytes); ok {
		return hb.FieldBytes()
	}
	var tmp byte
	var bitsfilled int
	switch pr.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < pr.Len(); i++ {
			out = append(out, reflectBytes(pr.Index(i))...)
		}
	case reflect.Struct:
		for i := 0; i < pr.NumField(); i++ {
			if w := pr.Type().Field(i).Tag.Get("width"); w != "" {
				// log.Println(pr.Type(), pr.Type().Field(0).Name)
				if pr.Type().Field(i).Type.Kind() != reflect.Int8 {
					log.Panicln("Programmer error, only int8 fields can have width tags")
				}
				wi, err := strconv.ParseInt(w, 0, 8)
				if err != nil {
					log.Panicln("Programmer error, bad int in field " + pr.Type().Field(i).Name)
				}
				// log.Printf("width %d oldtmp %x int was %d postshift was %v", wi, tmp, pr.Field(i).Int(), pr.Field(i).Int()<<uint64(8-bitsfilled-int(wi)))
				tmp |= byte(pr.Field(i).Int() << uint64(8-bitsfilled-int(wi)))
				// log.Printf("tmp %x", tmp)
				bitsfilled += int(wi)
				// log.Println("bitsfilled", bitsfilled)
				if bitsfilled > 8 {
					log.Panicln("bitsfilled must never get larger than 8, was", strconv.FormatInt(int64(bitsfilled), 10),
						"field was", pr.Type().Field(i).Name)
				} else if bitsfilled == 8 {
					// log.Println("old out", len(out))
					out = append(out, tmp)
					// log.Println("new out", len(out))
					tmp = 0
					bitsfilled = 0
				}
			} else if l := pr.Type().Field(i).Tag.Get("length"); l != "" {

				if pr.Type().Field(i).Type.Kind() != reflect.String {
					log.Panic("Programmer error, only int8 fields can have width tags")
				}
				li, err := strconv.ParseInt(l, 0, 8)
				if err != nil {
					log.Panic("Programmer error, bad int in field " + pr.Type().Field(i).Name)
				}
				b := []byte(pr.Field(i).String())
				for len(b) < int(li) {
					b = append(b, ' ')
				}
				// log.Printf("(%s)", b)
				out = append(out, b...)

			} else {
				out = append(out, reflectBytes(pr.Field(i))...)
			}

		}
	case reflect.String:
		out = []byte(pr.String())
	case reflect.Int8:
		out = []byte{byte(pr.Int())}
	}
	return out
}

func GetDevices() (devs []*portmidi.DeviceInfo) {
	dev_count := portmidi.CountDevices()
	devs = make([]*portmidi.DeviceInfo, dev_count)
	for i := 0; i < dev_count; i++ {
		devinfo := portmidi.Info(portmidi.DeviceID(i))
		devs[i] = devinfo
	}
	return
}

func OpenStream(output portmidi.DeviceID) (*Stream, error) {
	// log.Println("opening device", output)
	s, err := portmidi.NewOutputStream(output, 1024, 0)
	// log.Println(s, err)
	if err != nil {
		return nil, err
	}
	return &Stream{s}, nil
}

func (b *Stream) SendPatch(p Patch) (e error) {
	msgs := p.Msgs()
	for i, m := range msgs {
		// log.Printf("%x", m[len(m)-9:])
		// log.Println("trying to write", len(m), "bytes")
		e = b.Stream.WriteSysExBytes(0, m)
		if e != nil {
			log.Println("message was ", m)
			log.Println("Error writing msg", i, ":", e)
			return
		}
	}
	return nil
}
