package engine

import "sort"

// ProcUse is one process observed holding network connections.
type ProcUse struct {
	Name  string
	Conns int
}

// ActiveProcs returns processes that currently hold network connections,
// keyed by process name with the number of connections. It is best-effort and
// never errors: on unsupported platforms it returns an empty map. No elevated
// privileges are required — it only enumerates sockets, not their byte counts.
func ActiveProcs() map[string]int {
	out := map[string]int{}
	collectActiveProcs(out)
	return out
}

// sortedProcUses turns the connection map into a stable, name-sorted slice.
func sortedProcUses(m map[string]int) []ProcUse {
	out := make([]ProcUse, 0, len(m))
	for name, n := range m {
		out = append(out, ProcUse{Name: name, Conns: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
