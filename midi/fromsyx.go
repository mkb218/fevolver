package midi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strconv"
)

func checksum(buffers ...[]byte) int8 {
	var sum int8
	for _, b := range buffers {
		for _, by := range b {
			sum += int8(by)
		}
	}
	return sum & 0x7f
}

func FromSYXFile(filename string) (*Patch, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	totalbuf, err := ioutil.ReadAll(f)
	f.Close()
	if err != nil {
		return nil, err
	}
	return FromByteArray(totalbuf)
}

func FromByteArray(totalbuf []byte) (outp *Patch, err error) {
	p := Patch{}
	br := bytes.NewReader(totalbuf)
	var msgno int
	defer func() {
		f, ferr := os.Create(fmt.Sprint("/Users/mkane/code/mine-go/src/fevolver/midi/msg", msgno, ".json"))
		if ferr != nil {
			log.Println(ferr)
			if err == nil {
				err = ferr
			}
			return
		}
		defer func() {
			terr := f.Close()
			if err == nil {
				err = terr
			}
		}()
		j := json.NewEncoder(f)
		ferr = j.Encode(p)
		if ferr != nil {
			log.Println(ferr)
			if err != nil {
				err = ferr
				return
			}
		}

	}()

	for br.Len() > 0 {
		header := make([]byte, 9)
		n, err := br.Read(header)
		if n != 9 {
			return nil, fmt.Errorf("short read on header! %d < %d", n, 9)
		} else if err != nil {
			return nil, err
		}

		if header[0] != 0xf0 || header[1] != 0x43 {
			return nil, fmt.Errorf("Incorrect magic number %x != 0xf043", header[0:2])
		}
		if header[3] != 0x5e {
			return nil, fmt.Errorf("Incorrect Model ID %x != 0x53", header[3])
		}

		datatype := int(header[6])<<16 | int(header[7])<<8 | int(header[8])
		if datatype != fseqaddr {
			bytecount := int(header[4])<<7 | int(header[5])
			// log.Println(bytecount)
			data := make([]byte, bytecount)
			// log.Println("bytecount", bytecount, "data len", br.Len())
			n, err = br.Read(data)
			if n != bytecount {
				return nil, fmt.Errorf("short read on data! %d < %d", n, bytecount)
			} else if err != nil {
				return nil, err
			}

			footer := make([]byte, 2)
			n, err = br.Read(footer)
			if n != 2 {
				return nil, fmt.Errorf("short read on footer! %d < 2", n)
			} else if err != nil {
				return nil, err
			}

			if ck := checksum(header[4:], data, footer[0:1]); ck != 0 {
				log.Printf("Bad checksum! %x != %x", ck, 0)
			}
			var leftoverdata []byte
			switch datatype {
			case perfcommonaddr:
				leftoverdata = fromBytes(data, reflect.ValueOf(&(p.PerfCommon)), 0)
			case voice1addr:
				leftoverdata = fromBytes(data, reflect.ValueOf(&(p.Voices[0])), 0)
			case voice2addr:
				leftoverdata = fromBytes(data, reflect.ValueOf(&(p.Voices[1])), 0)
			case voice3addr:
				leftoverdata = fromBytes(data, reflect.ValueOf(&(p.Voices[2])), 0)
			case voice4addr:
				leftoverdata = fromBytes(data, reflect.ValueOf(&(p.Voices[3])), 0)
			default:
				return nil, fmt.Errorf("unknown datatype %x", datatype)
			}
			if len(leftoverdata) > 0 {
				return nil, fmt.Errorf("Failed to consume all data %x", leftoverdata)
			}
		} else {
			// log.Println("left in buffer before fseq", br.Len())
			ck, err := fseqFromBytes(br, &(p.FSEQ))
			if err != nil {
				return nil, err
			}
			footer, err := ioutil.ReadAll(br)
			if err != nil {
				return nil, err
			}
			// log.Println("footer", len(footer), "should be", FooterLen)
			ck += checksum(header[4:], footer[0:1])
			if ck != 0 {
				log.Printf("Bad checksum! %x != 0", ck)
			}
		}

		msgno++
	}

	return &p, nil

}

const fseqheaderlen = 32

func fseqFromBytes(reader *bytes.Reader, fseq *FSEQ) (checksum int8, err error) {
	header := make([]byte, fseqheaderlen)
	var n int
	n, err = reader.Read(header)
	for _, i := range header {
		checksum += int8(i)
	}
	if n < fseqheaderlen {
		err = fmt.Errorf("Didn't get enough bytes from FSEQ header, %d < %d", n, fseqheaderlen)
		return
	}
	if err != nil {
		return
	}
	fseq.Name = string(header[:8])
	// log.Println(len(header))
	header = header[16:] // skip 8 reserved bytes
	// log.Println(len(header))
	fseq.StartStepLoopPoint, fseq.EndStepLoopPoint = Int14(int(header[0])<<7|int(header[1])),
		Int14(int(header[2])<<7|int(header[3]))
	header = header[4:]
	// log.Println(len(header))

	fseq.LoopMode, fseq.SpeedAdjust, fseq.TempoVelocitySens, fseq.FormantPitchMode =
		int8(header[0]), int8(header[1]), int8(header[2]), int8(header[3])
	header = header[4:]
	// log.Println(len(header))

	fseq.FormantNoteAssign, fseq.FormantPitchTuning, fseq.FormantSequenceDelay, fseq.FrameDataFormat =
		int8(header[0]), int8(header[1]), int8(header[2]), int8(header[3])
	header = header[6:] // 2 more reserved bytes
	// log.Println(len(header))

	fseq.EndStepValidData = Int14(int(header[0])<<7 | int(header[1]))
	// log.Println(len(header))

	if fseq.FrameDataFormat < 0 || fseq.FrameDataFormat > 3 {
		err = fmt.Errorf("Unknown fseq Frame Data Format %d", fseq.FrameDataFormat)
		return
	}
	// log.Println("dataformat", fseq.FrameDataFormat, "bytes left in reader", reader.Len())

	fseq.FseqFrames = make([]FseqFrame, int(fseq.FrameDataFormat+1)*128)
	for i := range fseq.FseqFrames {
		data := make([]byte, 50)
		n, err = reader.Read(data)
		for _, i := range data {
			checksum += int8(i)
		}
		if n < 49 {
			err = fmt.Errorf("short read from fseq frame data frame %v got %v bytes", i, n)
			return
		} else if err != nil {
			err = fmt.Errorf("error read from fseq frame data %v %v", i, err)
			return
		}
		leftover := fromBytes(data, reflect.ValueOf(&(fseq.FseqFrames[i])), 0)
		if len(leftover) > 0 {
			err = fmt.Errorf("didn't consume all data from fseq %d", i)
			return
		}
	}
	err = nil
	return
}

func fromBytes(data []byte, rv reflect.Value, size int) []byte {
	// log.Printf("%v %v %v %.16x", rv.Type(), len(data), cap(data), data)
	defer func() {
		// log.Printf("(%v)", rv.Elem().Interface())
		if r := recover(); r != nil {
			log.Panicln(data[:16], rv, r)
		}
	}()
	if rv.Type().Kind() != reflect.Ptr {
		log.Panicln("fromBytes must get a pointer, got instead a", rv.Type())
	}
	switch rv.Elem().Kind() {
	case reflect.Struct:
		var sizeofbyte = 8
		for i := 0; i < rv.Elem().Type().NumField(); i++ {
			var size int
			// log.Println(rv.Elem().Type().Field(i).Name)
			switch rv.Elem().Field(i).Kind() {
			case reflect.Int8:
				if w := rv.Elem().Type().Field(i).Tag.Get("width"); w != "" {
					s, err := strconv.ParseInt(w, 0, 8)
					size = int(s)
					sizeofbyte -= int(size)
					if err != nil {
						log.Panicln("error parsing int from ", w)
					}
				}
			case reflect.String:
				if l := rv.Elem().Type().Field(i).Tag.Get("length"); l != "" {
					s, err := strconv.ParseInt(l, 0, 8)
					size = int(s)
					if err != nil {
						log.Panicln("error parsing int from ", l)
					}
				}
			}

			data = fromBytes(data, rv.Elem().Field(i).Addr(), size)
			if sizeofbyte == 0 {
				data = data[1:]
				sizeofbyte = 8
			}
		}
		return data
	case reflect.Int8:
		if size != 0 {
			// log.Printf("size is %d, first byte of data is %x", size, data[0])
			// log.Printf("value assigned is %x", int8(data[0]>>(8-uint(size))))
			rv.Elem().Set(reflect.ValueOf(int8(data[0] >> (8 - uint(size)))))
			data[0] <<= uint(size)
			// log.Printf("now first byte of data is %x", data[0])
			return data
		} else {
			rv.Elem().Set(reflect.ValueOf(int8(data[0])))
			return data[1:]
		}
	case reflect.String:
		rv.Elem().Set(reflect.ValueOf(string(data[:size])))
		return data[size:]
	case reflect.Array, reflect.Slice:
		for i := 0; i < rv.Elem().Len(); i++ {
			data = fromBytes(data, rv.Elem().Index(i).Addr(), 0)
		}
		return data
	default:
		if rv.Elem().Type().Name() == "ReservedBits" {
			return data[1:]
		} else if rv.Elem().Type().Name() == "Int14" {
			rv.Elem().Set(reflect.ValueOf(Int14(data[0])<<7 | Int14(data[1])))
			return data[2:]
		}
		log.Panicln("unknown type", rv.Type())
	}
	log.Panicln("This should be unreachable!")
	return nil
}
