package compat

import (
	"fmt"
	"regexp"
	"strings"

	"mosaic-run/internal/domain"
)

// classifySchedules implements A7 and fills the seed classification, producing
// rows and in-block writer rows of every block of a 2.16 table.
//
// A block's Source is a seed (the first row's Inputs list it, or the first row
// begins the block) or is output by a row outside the block that runs before
// the block is entered by the declared On Success, NEXT, block entry and exit
// routes. Any other Source is refused.
func classifySchedules(table domain.RoutingTable, blocks []domain.AdmittedBlock) error {
	graph := runGraph(table, blocks)
	nodes := blockNodes(len(table.Rows), blocks)
	for i := range blocks {
		b := &blocks[i]
		b.WriterRows = writerRows(table, *b)
		if isSeed(table, *b) {
			b.Seed = domain.SeedSeeded
			b.ProducingRows = nil
			continue
		}
		b.ProducingRows = producersBeforeEntry(table, *b, graph, nodes)
		if len(b.ProducingRows) == 0 {
			return refusal(table, "A7", domain.WorkflowRowFromIndex(b.StartRow), b.Source,
				fmt.Sprintf("block %q: Source %q is neither a seed nor output by a row outside the block that runs before it is entered", b.Name, b.Source))
		}
		b.Seed = domain.SeedProduced
	}
	return nil
}

// isSeed reports whether the first row begins b or lists its Source as an Input.
func isSeed(table domain.RoutingTable, b domain.AdmittedBlock) bool {
	if b.StartRow == 0 {
		return true
	}
	for _, in := range table.Rows[0].InputArtifacts {
		if pathMatches(in, b.Source) {
			return true
		}
	}
	return false
}

// outputsSource reports whether an Output entry of row produces source.
func outputsSource(row domain.RoutingRow, source string) bool {
	for _, out := range row.OutputArtifacts {
		if pathMatches(out, source) {
			return true
		}
	}
	return false
}

// writerRows returns the rows of b (zero-based, table order) that output its Source.
func writerRows(table domain.RoutingTable, b domain.AdmittedBlock) []int {
	var rows []int
	for i := b.StartRow; i < b.EndRow; i++ {
		if outputsSource(table.Rows[i], b.Source) {
			rows = append(rows, i)
		}
	}
	return rows
}

// producersBeforeEntry returns the rows outside b (zero-based, table order)
// that output its Source and run before b is first entered: their run-scope
// node is reached from the start of the run without passing through b, and b
// is reachable from it.
func producersBeforeEntry(table domain.RoutingTable, b domain.AdmittedBlock, graph routeGraph, nodes []int) []int {
	entry := b.StartRow
	before := map[int]bool{}
	queue := []int{nodes[0]}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if before[n] || n == entry {
			continue
		}
		before[n] = true
		queue = append(queue, graph[n]...)
	}

	reverse := routeGraph{}
	for from, tos := range graph {
		for _, to := range tos {
			reverse[to] = append(reverse[to], from)
		}
	}
	leadsToEntry := map[int]bool{}
	queue = []int{entry}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if leadsToEntry[n] {
			continue
		}
		leadsToEntry[n] = true
		queue = append(queue, reverse[n]...)
	}

	var rows []int
	for i, row := range table.Rows {
		if b.Contains(i) || !outputsSource(row, b.Source) {
			continue
		}
		if before[nodes[i]] && leadsToEntry[nodes[i]] {
			rows = append(rows, i)
		}
	}
	return rows
}

// pathMatches reports whether pattern, in which `*` stands for any text within
// one path segment, matches path exactly.
func pathMatches(pattern, path string) bool {
	if !strings.Contains(pattern, "*") {
		return pattern == path
	}
	parts := strings.Split(pattern, "*")
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	re, err := regexp.Compile("^" + strings.Join(parts, "[^/]*") + "$")
	return err == nil && re.MatchString(path)
}
