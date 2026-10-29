package qe

import (
	"io"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func (c *equalityDERCall) preparePositions(n int) error {
	if n > cap(c.positions) {
		bytes := uint64(n-cap(c.positions)) * uint64(reflect.TypeOf(int(0)).Size())
		if err := c.owner.manager.Budget().Reserve(bytes); err != nil {
			return err
		}
		c.bytes += bytes
		c.positions = make([]int, n)
	} else {
		c.positions = c.positions[:n]
	}
	for i := range c.positions {
		c.positions[i] = -1
	}
	c.definitions = c.definitions[:0]
	c.order = c.order[:0]
	return nil
}
func (c *equalityDERCall) ensureDefinition(index uint32) error {
	if index == ^uint32(0) {
		return arena.ErrBounds
	}
	n := int(uint64(index) + 1)
	if uint64(n) != uint64(index)+1 {
		return arena.ErrBounds
	}
	if n <= len(c.definitions) {
		return nil
	}
	old := len(c.definitions)
	if n > cap(c.definitions) {
		bytes := uint64(n-cap(c.definitions)) * uint64(reflect.TypeOf(equalityDefinition{}).Size())
		if err := c.owner.manager.Budget().Reserve(bytes); err != nil {
			return err
		}
		c.bytes += bytes
		next := make([]equalityDefinition, n)
		copy(next, c.definitions)
		c.definitions = next
	} else {
		c.definitions = c.definitions[:n]
	}
	for i := old; i < n; i++ {
		c.definitions[i] = equalityDefinition{position: -1}
	}
	return nil
}
func (d *EqualityDER) uninterpretedConstant(h ast.ASTHandle) (bool, error) {
	kind, err := d.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	n, err := d.manager.GetNumArgs(h)
	if err != nil || n != 0 {
		return false, err
	}
	family, err := d.manager.GetFamilyID(h)
	return family == ast.NullFamilyID, err
}
func (d *EqualityDER) findDefinitions(c *equalityDERCall, args []ast.ASTHandle, exists bool) (count uint32, err error) {
	if err = c.preparePositions(len(args)); err != nil {
		return 0, err
	}
	for i, h := range args {
		if err = d.checkpoint(); err != nil {
			return count, err
		}
		solutions, matched, err := d.variableDefinition(c, h, exists)
		if err != nil {
			return count, err
		}
		if !matched {
			continue
		}
		for _, solution := range solutions {
			index, err := d.manager.GetVarIndex(solution.variable)
			if err != nil {
				return count, err
			}
			if err = c.ensureDefinition(index); err != nil {
				return count, err
			}
			entry := &c.definitions[index]
			if entry.term.IsNull() {
				if _, err = c.hold(solution.term); err != nil {
					return count, err
				}
				*entry = equalityDefinition{solution.variable, solution.term, i}
				c.positions[i] = int(index)
				count++
				continue
			}
			value, err := d.manager.IsValue(entry.term)
			if err != nil {
				return count, err
			}
			if value {
				continue
			}
			previous := entry.term
			preferred, err := d.manager.IsValue(solution.term)
			if err != nil {
				return count, err
			}
			if !preferred {
				ground, err := d.manager.IsGround(solution.term)
				if err != nil {
					return count, err
				}
				oldGround, err := d.manager.IsGround(previous)
				if err != nil {
					return count, err
				}
				preferred = ground && !oldGround
			}
			if !preferred {
				preferred, err = d.uninterpretedConstant(solution.term)
				if err != nil {
					return count, err
				}
			}
			if preferred {
				if _, err = c.hold(solution.term); err != nil {
					return count, err
				}
				c.positions[entry.position] = -1
				c.positions[i] = int(index)
				entry.position = i
				entry.term = solution.term
			}
			// Source emits this diagnostic for every nonvalue incumbent,
			// including candidates rejected by all preference rules.
			err = d.emit("qe_def", "find_definitions", 625, func(out io.Writer) error {
				_, _ = io.WriteString(out, "Replacing definition of VAR "+strconv.FormatUint(uint64(index), 10)+" from ")
				if err := writeExpression(d.manager, d.config, out, previous); err != nil {
					return err
				}
				_, _ = io.WriteString(out, " to ")
				if err := writeExpression(d.manager, d.config, out, solution.term); err != nil {
					return err
				}
				_, _ = io.WriteString(out, " inferred from: ")
				if err := writeExpression(d.manager, d.config, out, h); err != nil {
					return err
				}
				_, _ = io.WriteString(out, "\n")
				return nil
			})
			if err != nil {
				return count, err
			}
		}
	}
	return count, nil
}

type equalitySortFrame struct {
	term ast.ASTHandle
	next uint32
}

func (d *EqualityDER) eliminationOrder(c *equalityDERCall) error {
	c.order = c.order[:0]
	if err := d.emit("top_sort", "get_elimination_order", 424, func(out io.Writer) error {
		_, _ = io.WriteString(out, "DEFINITIONS: ")
		_ = diagnosticEndl(out)
		for i, entry := range c.definitions {
			if entry.term.IsNull() {
				continue
			}
			_, _ = io.WriteString(out, "VAR "+strconv.Itoa(i)+" = ")
			if err := writeExpression(d.manager, d.config, out, entry.term); err != nil {
				return err
			}
			_ = diagnosticEndl(out)
		}
		return nil
	}); err != nil {
		return err
	}
	found := false
	for i := range c.definitions {
		entry := &c.definitions[i]
		if entry.term.IsNull() {
			continue
		}
		quantified, err := d.manager.HasQuantifiers(entry.term)
		if err != nil {
			return err
		}
		occurs := false
		if !quantified {
			occurs, err = OccursVariable(d.manager, uint32(i), entry.term)
			if err != nil {
				return err
			}
		}
		if quantified || occurs {
			entry.term = ast.ASTHandle{}
		} else {
			found = true
		}
	}
	if found {
		visiting := make(map[ast.ASTHandle]bool)
		done := make(map[ast.ASTHandle]bool)
		for i := range c.definitions {
			if c.definitions[i].term.IsNull() {
				continue
			}
			stack := []equalitySortFrame{{term: c.definitions[i].variable}}
			for len(stack) != 0 {
				last := len(stack) - 1
				frame := &stack[last]
				count, err := d.manager.GetRefCount(frame.term)
				if err != nil {
					return err
				}
				shared := count > 1
				if shared && done[frame.term] {
					stack = stack[:last]
					continue
				}
				kind, err := d.manager.GetKind(frame.term)
				if err != nil {
					return err
				}
				switch kind {
				case ast.ASTVar:
					index, err := d.manager.GetVarIndex(frame.term)
					if err != nil {
						return err
					}
					if frame.next == 0 && uint64(index) >= uint64(len(c.definitions)) {
						if err := d.emit("der_bug", "der_sort_vars", 127, func(out io.Writer) error {
							_, _ = io.WriteString(out, "vidx: "+strconv.FormatUint(uint64(index), 10)+"\n")
							return nil
						}); err != nil {
							return err
						}
					}
					candidate := uint64(index) < uint64(len(c.definitions)) && !c.definitions[index].term.IsNull()
					if frame.next == 0 && candidate {
						if visiting[frame.term] {
							visiting[frame.term] = false
							c.definitions[index].term = ast.ASTHandle{}
						} else {
							visiting[frame.term] = true
							frame.next = 1
							stack = append(stack, equalitySortFrame{term: c.definitions[index].term})
							continue
						}
					} else if frame.next == 1 && candidate {
						visiting[frame.term] = false
						c.order = append(c.order, index)
					}
					if shared {
						done[frame.term] = true
					}
					stack = stack[:last]
				case ast.ASTApp:
					n, err := d.manager.GetNumArgs(frame.term)
					if err != nil {
						return err
					}
					if frame.next < n {
						arg, err := d.manager.GetArg(frame.term, frame.next)
						if err != nil {
							return err
						}
						frame.next++
						refs, err := d.manager.GetRefCount(arg)
						if err != nil {
							return err
						}
						if refs > 1 && done[arg] {
							continue
						}
						stack = append(stack, equalitySortFrame{term: arg})
						continue
					}
					if shared {
						done[frame.term] = true
					}
					stack = stack[:last]
				default:
					return ast.ErrASTDomain
				}
			}
		}
	}
	return d.emit("qe_lite", "get_elimination_order", 432, func(out io.Writer) error {
		_, _ = io.WriteString(out, "Elimination m_order:")
		_ = diagnosticEndl(out)
		for i, index := range c.order {
			if i != 0 {
				_, _ = io.WriteString(out, ",")
			}
			_, _ = io.WriteString(out, strconv.FormatUint(uint64(index), 10))
		}
		_ = diagnosticEndl(out)
		return nil
	})
}
