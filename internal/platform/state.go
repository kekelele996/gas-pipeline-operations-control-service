package platform

import (
	"fmt"
	"sort"
)

// StateMachine is a small, explicit transition table. It encodes which target
// states are reachable from each source state. Transitions not listed are
// illegal and Transition returns ErrState.
//
// The table is read-only after construction, so it is safe for concurrent use
// by any number of goroutines.
type StateMachine struct {
	name    string
	table   map[string]map[string]struct{}
	initial string
}

// NewStateMachine builds a state machine from a map of source -> []allowed
// target states. The map is copied internally.
func NewStateMachine(name, initial string, table map[string][]string) *StateMachine {
	m := &StateMachine{
		name:    name,
		table:   make(map[string]map[string]struct{}, len(table)),
		initial: initial,
	}
	for src, dsts := range table {
		set := make(map[string]struct{}, len(dsts))
		for _, d := range dsts {
			set[d] = struct{}{}
		}
		m.table[src] = set
	}
	return m
}

// Initial returns the machine's initial state.
func (m *StateMachine) Initial() string { return m.initial }

// States returns the full set of states known to the machine, sorted.
func (m *StateMachine) States() []string {
	seen := make(map[string]struct{})
	for src, dsts := range m.table {
		seen[src] = struct{}{}
		for d := range dsts {
			seen[d] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// CanTransition reports whether transitioning from src to dst is legal.
func (m *StateMachine) CanTransition(src, dst string) bool {
	if dsts, ok := m.table[src]; ok {
		_, ok := dsts[dst]
		return ok
	}
	return false
}

// Transition validates and returns the target state. It returns ErrState when
// the move is not allowed, and ErrInvalid when src is unknown.
func (m *StateMachine) Transition(src, dst string) (string, error) {
	dsts, ok := m.table[src]
	if !ok {
		return src, Statef("%s: unknown source state %q", m.name, src)
	}
	if _, ok := dsts[dst]; !ok {
		return src, Statef("%s: cannot transition from %q to %q", m.name, src, dst)
	}
	return dst, nil
}

// Allowed returns the sorted list of states reachable from src.
func (m *StateMachine) Allowed(src string) []string {
	dsts, ok := m.table[src]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(dsts))
	for d := range dsts {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// Must panics if err is non-nil; used only in package-level table construction.
func Must(err error) {
	if err != nil {
		panic(fmt.Sprintf("platform state machine: %v", err))
	}
}

// EnsureStates asserts that every state in states is reachable somewhere in
// the table; it panics during construction if a declared state is missing.
func (m *StateMachine) EnsureStates(states ...string) {
	for _, s := range states {
		if _, ok := m.table[s]; !ok {
			// a state may only appear as a target — that's fine
			found := false
			for _, dsts := range m.table {
				if _, ok := dsts[s]; ok {
					found = true
					break
				}
			}
			if !found {
				panic(fmt.Sprintf("%s: state %q is not in the transition table", m.name, s))
			}
		}
	}
}
