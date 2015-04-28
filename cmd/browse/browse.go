package main

import (
	"encoding/gob"
	"encoding/json"
	"fevolver/cmd/common"
	"fevolver/midi"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/rakyll/portmidi"
)

func main() {
	statefile := flag.String("state", "state.gob", "(optional) Location for temp audio files (must exist)")
	mididevice := flag.Int("mididev", -1, "MIDI device")
	flag.Parse()
	f, err := os.Open(*statefile)
	if err != nil {
		log.Panic(err)
	}
	g := gob.NewDecoder(f)
	var s common.State
	err = g.Decode(&s)
	if err != nil {
		log.Panic(err)
	}
	midistream, err := midi.OpenStream(portmidi.DeviceID(*mididevice))
	if err != nil {
		log.Panic(err)
	}
	for {
		var g string
		fmt.Printf("Please type a generation number (0-%d), or q to quit\n", len(s.Generations)-1)
		fmt.Scan(&g)
		if g == "q" {
			break
		}
		var gi int64
		var err error
		if gi, err = strconv.ParseInt(g, 10, 32); err != nil || int(gi) >= len(s.Generations) {
			fmt.Println("Invalid integer:", g, err)
			continue
		}
		for {
			for i, p := range s.Generations[int(gi)].Patches {
				fmt.Printf("%d.) %s FSEQ part %d len %d\n", i, p.Name, p.FseqPart, len(p.FSEQ.FseqFrames))
			}
			fmt.Printf("Please type an individual number (0-%d), or q to quit\n", len(s.Generations[int(gi)].Patches)-1)
			fmt.Scan(&g)
			if g == "q" {
				break
			}
			var ii int64
			if ii, err = strconv.ParseInt(g, 10, 32); err != nil || int(ii) >= len(s.Generations[gi].Patches) {
				fmt.Println("Invalid integer:", g, err)
				continue
			}
			if err = midistream.SendPatch(s.Generations[gi].Patches[ii].Patch); err != nil {
				fmt.Println("Error sending patch!", err)
				return
			}

			jfile, err := os.Create(fmt.Sprintf("g%dp%d.json", gi, ii))
			if err != nil {
				fmt.Println("Couldn't open output file!", err)
				continue
			}
			func() {
				defer func() {
					err := jfile.Close()
					if err != nil {
						fmt.Println("Error when closing json file!", err)
					}
				}()
				b, err := json.MarshalIndent(s.Generations[gi].Patches[ii], "", "\t")
				if err != nil {
					fmt.Println("Error marshalling JSON!", err)
					return
				}
				_, err = jfile.Write(b)
				if err != nil {
					fmt.Println("Error writing JSON!", err)
					return
				}
			}()

		}

	}

}
