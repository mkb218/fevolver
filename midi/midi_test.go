package midi

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
)

func init() {
	rand.Seed(0)
}

func TestRandomPatch(t *testing.T) {
	p := RandomPatch()
	p.PerfCommon.Name = "TESTPATCH   "
	p.FSEQ.Name = "TESTFSEQ"
	p.Voices[0].Name = "TESTVOICE1"
	p.Voices[1].Name = "TESTVOICE2"
	p.Voices[2].Name = "TESTVOICE3"
	p.Voices[3].Name = "TESTVOICE4"
	Mutate(p, 0)

}

func TestFromSYX(t *testing.T) {
	p, err := FromSYXFile("/Users/mkane/code/non-echonest/mine-go/src/fevolver/midi/Untitled.syx")
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	outfile, err := os.Create("/Users/mkane/code/non-echonest/mine-go/src/fevolver/midi/Untitled-out.syx")
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	defer func() {
		err = outfile.Close()
		if err != nil {
			t.Log(err)
			t.Fail()
		}
	}()
	msgs := p.Msgs()
	for i, m := range msgs {
		_, err := outfile.Write(m)
		if err != nil {
			t.Log(err)
			t.Fail()
		}
		singlemsg, err := os.Create(fmt.Sprint("msg", i, ".syx"))
		if err != nil {
			t.Log(err)
			t.Fail()
			continue
		}
		defer singlemsg.Close()
		_, err = singlemsg.Write(m)
		if err != nil {
			t.Log(err)
			t.Fail()
		}
	}
	// _, err = FromSYXFile("/Users/mkane/code/non-echonest/mine-go/src/fevolver/midi/Untitled-out.syx")
	// if err != nil {
	// 	t.Log(err)
	// 	t.FailNow()
	// }

}

// func TestCrossover(t *testing.T) {
// 	outfile, err := os.Create("/Users/mkane/code/non-echonest/mine-go/src/fevolver/midi/jsonlog.json")
// 	if err != nil {
// 		t.Log(err)
// 		t.FailNow()
// 	}
// 	defer outfile.Close()

// 	// j := json.NewEncoder(outfile)

// 	mom, dad := RandomPatch(), RandomPatch()
// 	t.Log("mom", mom.FSEQ.FrameDataFormat, len(mom.FSEQ.FseqFrames))
// 	t.Log("dad", dad.FSEQ.FrameDataFormat, len(dad.FSEQ.FseqFrames))

// 	for i := 0; i < 10; i++ {
// 		child1, child2, err := Crossover(&mom, &dad)
// 	}
// }
