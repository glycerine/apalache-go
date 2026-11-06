package interp

// Source: src/interp/iz3profiling.cpp; src/interp/iz3profiling.h.

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

type profileNode struct {
	name     string
	elapsed  time.Duration
	started  time.Time
	parent   *profileNode
	children map[string]*profileNode
	order    []string
}

// Profiler owns a nested interpolation timer tree. NewProfiler creates an
// independent tree; source-facing clients use the process-lifetime tree below.
type Profiler struct {
	top, current *profileNode
	now          func() time.Time
	origin       time.Time
}

func NewProfiler() *Profiler {
	top := &profileNode{name: "TOTAL", children: make(map[string]*profileNode)}
	now := time.Now
	return &Profiler{top: top, current: top, now: now, origin: now()}
}

// ProcessProfiler returns the source-facing process-lifetime interpolation
// timer owner. Like the source profiler, it assumes single-goroutine use.
func ProcessProfiler() *Profiler { return processProfiler }

var processProfiler = NewProfiler()

func (p *Profiler) TimerStart(name string) error {
	if p == nil || p.top == nil || p.current == nil || name == "" {
		return NewError("invalid interpolation profiler")
	}
	child := p.current.children[name]
	if child == nil {
		child = &profileNode{name: name, parent: p.current, children: make(map[string]*profileNode)}
		p.current.children[name] = child
		p.current.order = append(p.current.order, name)
	}
	child.started = p.now()
	p.current = child
	return nil
}
func (p *Profiler) TimerStop(name string) error {
	if p == nil || p.current == nil || p.current.parent == nil || p.current.name != name {
		return NewError("imbalanced timer_start and timer_stop")
	}
	p.current.elapsed += p.now().Sub(p.current.started)
	p.current = p.current.parent
	return nil
}
func profileSeconds(d time.Duration) string { return fmt.Sprintf("%g", d.Seconds()) }
func (p *Profiler) printNode(w io.Writer, node *profileNode, indent int, totals map[string]time.Duration) error {
	prefix := strings.Repeat("  ", indent) + node.name
	dots := 70 - 2*indent - len(node.name)
	if dots < 0 {
		dots = 0
	}
	if _, err := fmt.Fprintf(w, "%s%s%s\n", prefix, strings.Repeat(".", dots), profileSeconds(node.elapsed)); err != nil {
		return err
	}
	if indent != 0 {
		totals[node.name] += node.elapsed
	}
	for _, name := range node.order {
		if err := p.printNode(w, node.children[name], indent+1, totals); err != nil {
			return err
		}
	}
	return nil
}
func (p *Profiler) Print(w io.Writer) error {
	if p == nil || p.top == nil || w == nil {
		return NewError("invalid interpolation profiler")
	}
	p.top.elapsed = 0
	for _, name := range p.top.order {
		p.top.elapsed += p.top.children[name].elapsed
	}
	totals := make(map[string]time.Duration)
	if err := p.printNode(w, p.top, 0, totals); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "TOTALS:\n"); err != nil {
		return err
	}
	names := make([]string, 0, len(totals))
	for name := range totals {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err := fmt.Fprintf(w, "%s %s\n", name, profileSeconds(totals[name])); err != nil {
			return err
		}
	}
	return nil
}
func (p *Profiler) ShowTime(w io.Writer) error {
	if p == nil || p.now == nil || p.origin.IsZero() || w == nil {
		return NewError("invalid interpolation profiler")
	}
	_, err := fmt.Fprintf(w, "%g\n", p.now().Sub(p.origin).Seconds())
	return err
}
