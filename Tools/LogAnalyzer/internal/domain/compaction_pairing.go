package domain

import "time"

// CompactionEvent is one logged compaction event of a single stream, in file
// order, as input to PairCompactionEvents.
type CompactionEvent struct {
	SchemaVersion string
	At            time.Time
	Fields        CompactionFields
}

// PairedCompaction is the result of pairing: the compaction (Actor left zero)
// and the positions of its halves in the input; Second is -1 when unpaired.
type PairedCompaction struct {
	Compaction Compaction
	First      int
	Second     int
}

// legacy reports whether the event is a phase-less event of a supported
// version below 1.5.0.
func (e CompactionEvent) legacy() bool {
	return e.Fields.Phase == CompactionPhaseNone && SchemaVersionBelow150(e.SchemaVersion)
}

// PairCompactionEvents turns the compaction events of one stream into
// compactions, ordered by the position of their first half.
//
// A phase "post" pairs with the nearest preceding unpaired "pre"; an unpaired
// half is one compaction. A phase-less event of a version below 1.5.0 is
// legacy: a legacy event with no tokens_after pairs with the next compaction
// event of the stream when that is also legacy and carries neither trigger nor
// tokens_before. Pairing is greedy left to right. Every other event is one
// self-contained compaction.
func PairCompactionEvents(evs []CompactionEvent) []PairedCompaction {
	second := make(map[int]int) // first half -> second half
	consumed := make(map[int]bool)
	var pres []int
	for i, e := range evs {
		switch {
		case consumed[i]:
		case e.Fields.Phase == CompactionPhasePre:
			pres = append(pres, i)
		case e.Fields.Phase == CompactionPhasePost:
			if n := len(pres); n > 0 {
				second[pres[n-1]] = i
				consumed[i] = true
				pres = pres[:n-1]
			}
		case e.legacy() && !e.Fields.TokensAfter.IsPresent() && i+1 < len(evs) && legacySecondHalf(evs[i+1]):
			second[i] = i + 1
			consumed[i+1] = true
		}
	}

	var out []PairedCompaction
	for i, e := range evs {
		if consumed[i] {
			continue
		}
		pc := PairedCompaction{First: i, Second: -1, Compaction: singleCompaction(e)}
		if j, ok := second[i]; ok {
			pc.Second = j
			pc.Compaction = pairedCompaction(e, evs[j])
		} else if e.Fields.Phase == CompactionPhasePre {
			pc.Compaction.Shape = CompactionPreOnly
		} else if e.Fields.Phase == CompactionPhasePost {
			pc.Compaction.Shape = CompactionPostOnly
		}
		out = append(out, pc)
	}
	return out
}

// legacySecondHalf reports whether e can complete a legacy pair.
func legacySecondHalf(e CompactionEvent) bool {
	return e.legacy() && e.Fields.Trigger == "" && !e.Fields.TokensBefore.IsPresent()
}

func singleCompaction(e CompactionEvent) Compaction {
	return Compaction{
		At:           e.At,
		Trigger:      e.Fields.Trigger,
		TokensBefore: e.Fields.TokensBefore,
		TokensAfter:  e.Fields.TokensAfter,
		Shape:        CompactionSingle,
	}
}

func pairedCompaction(first, second CompactionEvent) Compaction {
	c := singleCompaction(first)
	c.Shape = CompactionPaired
	c.EndedAt = second.At
	c.TokensAfter = second.Fields.TokensAfter
	if c.Trigger == "" {
		c.Trigger = second.Fields.Trigger
	}
	return c
}
