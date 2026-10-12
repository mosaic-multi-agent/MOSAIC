package compat

import (
	"fmt"

	"mosaic-run/internal/domain"
)

// routeGraph is a directed graph over integer nodes. In the run scope a node is
// a row index, except that a whole block is one node identified by its first
// row. Inside one group or ungrouped iteration a node is a row index.
type routeGraph map[int][]int

// blockNodes maps each row to its run-scope node: the first row of its block,
// or the row itself outside every block.
func blockNodes(nRows int, blocks []domain.AdmittedBlock) []int {
	nodes := make([]int, nRows)
	for i := range nodes {
		nodes[i] = i
	}
	for _, b := range blocks {
		for i := b.StartRow; i < b.EndRow; i++ {
			nodes[i] = b.StartRow
		}
	}
	return nodes
}

// nodeEdges returns the declared routes out of row: its On Success targets,
// fork entries, block entry, or NEXT. in maps a row to its node and reports
// whether the row belongs to the graph.
func nodeEdges(table domain.RoutingTable, blocks []domain.AdmittedBlock, row int, in func(int) (int, bool)) []int {
	var out []int
	add := func(r int) {
		if n, ok := in(r); ok {
			out = append(out, n)
		}
	}
	s := table.Rows[row].Success
	switch s.Kind {
	case domain.SuccessTarget:
		add(s.Target.Index)
	case domain.SuccessFork:
		for _, f := range s.Fork {
			add(f.Index)
		}
	case domain.SuccessBlockEntry:
		for _, b := range blocks {
			if b.Name == s.Block {
				add(b.StartRow)
			}
		}
	case domain.SuccessNext:
		add(row + 1)
	}
	return out
}

// runGraph builds the run-scope route graph: run-scope rows route by On
// Success, NEXT and block entry, and a block (one node) leaves to the row below
// its last row.
func runGraph(table domain.RoutingTable, blocks []domain.AdmittedBlock) routeGraph {
	n := len(table.Rows)
	nodes := blockNodes(n, blocks)
	in := func(r int) (int, bool) {
		if r < 0 || r >= n {
			return 0, false
		}
		return nodes[r], true
	}
	g := routeGraph{}
	for i := 0; i < n; i++ {
		if nodes[i] != i {
			continue
		}
		isBlock := false
		for _, b := range blocks {
			if b.StartRow == i {
				isBlock = true
				if edge, ok := in(b.EndRow); ok {
					g[i] = append(g[i], edge)
				}
			}
		}
		if !isBlock {
			g[i] = nodeEdges(table, blocks, i, in)
		}
	}
	return g
}

// groupGraphs builds one route graph per group (or ungrouped iteration) of every
// block. Targets and NEXT stay inside the group; leaving it adds no edge.
func groupGraphs(table domain.RoutingTable, blocks []domain.AdmittedBlock) []routeGraph {
	var graphs []routeGraph
	for _, b := range blocks {
		for _, grp := range b.Groups {
			start, end := grp.StartRow, grp.EndRow
			in := func(r int) (int, bool) { return r, r >= start && r < end }
			g := routeGraph{}
			for i := start; i < end; i++ {
				g[i] = nodeEdges(table, blocks, i, in)
			}
			graphs = append(graphs, g)
		}
	}
	return graphs
}

// findCycle returns a node on a cycle of g, and true. Nodes are visited in
// ascending order so the result is deterministic.
func findCycle(g routeGraph, nRows int) (int, bool) {
	const (
		white = iota
		grey
		black
	)
	color := make(map[int]int, len(g))
	var visit func(n int) (int, bool)
	visit = func(n int) (int, bool) {
		color[n] = grey
		for _, m := range g[n] {
			switch color[m] {
			case grey:
				return m, true
			case white:
				if at, found := visit(m); found {
					return at, true
				}
			}
		}
		color[n] = black
		return 0, false
	}
	for n := 0; n < nRows; n++ {
		if _, ok := g[n]; ok && color[n] == white {
			if at, found := visit(n); found {
				return at, true
			}
		}
	}
	return 0, false
}

// checkNoCycles implements the cycle part of A15 for engine-routed modes: the
// declared On Success, NEXT, block entry and exit routes of each scope contain
// no cycle. On Findings routes do not count. Orchestrated mode is unchanged.
func checkNoCycles(table domain.RoutingTable, blocks []domain.AdmittedBlock, mode domain.ExecutionMode) error {
	if mode == domain.ExecutionModeOrchestrated {
		return nil
	}
	n := len(table.Rows)
	graphs := append([]routeGraph{runGraph(table, blocks)}, groupGraphs(table, blocks)...)
	for _, g := range graphs {
		if at, found := findCycle(g, n); found {
			return refusal(table, "A15", domain.WorkflowRowFromIndex(at), "",
				fmt.Sprintf("the declared routes contain a cycle through row %d: the engine would route it forever", at+1))
		}
	}
	return nil
}
