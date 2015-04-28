package common

import (
	"fevolver/midi"

	"github.com/mkb218/gosndfile/sndfile"
)

type State struct {
	Generations []Generation
	SourceAudio []float32
	Format      sndfile.Info
}

type ScoredPatch struct {
	midi.Patch
	Score float64
	Filtered bool
	Audio []float32
}

type Generation struct {
	Number  int
	Patches []ScoredPatch
}

func (g *Generation) Len() int {
	// log.Println("Generation", g.Number, "has", len(g.Patches))
	return len(g.Patches)
}

func (g *Generation) Less(i, j int) bool {
	// log.Println(g.Patches[i].Score, ">", g.Patches[j].Score)
	return g.Patches[i].Score > g.Patches[j].Score
}

func (g *Generation) Swap(i, j int) {
	// log.Println("swap", i, g.Patches[i].Score, j, g.Patches[j].Score)
	g.Patches[i], g.Patches[j] = g.Patches[j], g.Patches[i]
}
