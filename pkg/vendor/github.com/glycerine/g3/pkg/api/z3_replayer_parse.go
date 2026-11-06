package api

import (
	"fmt"
)

func (r *Z3Replayer) Parse() error {
	if r == nil || r.closed || r.scanner == nil {
		return ErrZ3ReplayState
	}
	s := r.scanner
	for {
		s.blanks()
		if s.eof {
			return nil
		}
		command := s.current
		s.next()
		s.blanks()
		var err error
		switch command {
		case 'V':
			_, err = s.quoted('"')
		case 'R':
			r.resetArguments()
		case 'P':
			var p uint64
			p, err = s.hexadecimal()
			if err == nil {
				err = r.pushObject(p)
			}
		case 'S':
			var v string
			v, err = s.quoted('"')
			if err == nil {
				r.args = append(r.args, z3ReplayValue{kind: replayString, s: v})
			}
		case 'N':
			r.args = append(r.args, z3ReplayValue{kind: replaySymbol, nullSymbol: true})
		case '$':
			var v string
			v, err = s.quoted('|')
			if err == nil {
				r.args = append(r.args, z3ReplayValue{kind: replaySymbol, s: v})
			}
		case '#':
			var v uint64
			v, err = s.uint64()
			if err == nil {
				r.args = append(r.args, z3ReplayValue{kind: replaySymbol, s: fmt.Sprint(v), u: v, numericSymbol: true})
			}
		case 'I':
			var v int64
			v, err = s.int64()
			if err == nil {
				r.args = append(r.args, z3ReplayValue{kind: replayInt64, i: v})
			}
		case 'U':
			var v uint64
			v, err = s.uint64()
			if err == nil {
				r.args = append(r.args, z3ReplayValue{kind: replayUint64, u: v})
			}
		case 'F':
			var v float64
			v, err = s.floating(32)
			if err == nil {
				r.args = append(r.args, z3ReplayValue{kind: replayFloat, f: float32(v)})
			}
		case 'D':
			var v float64
			v, err = s.floating(64)
			if err == nil {
				r.args = append(r.args, z3ReplayValue{kind: replayDouble, d: v})
			}
		case 'p', 's', 'u':
			var n uint64
			n, err = s.uint64()
			if err == nil {
				kind := replayObject
				if command == 's' {
					kind = replaySymbol
				}
				if command == 'u' {
					kind = replayUint64
				}
				err = r.pushArray(n, kind)
			}
		case 'C':
			var id uint64
			id, err = s.uint64()
			if err == nil {
				entry, found := r.commands[uint32(id)]
				if !found {
					err = r.failure("invalid command")
				} else {
					err = entry.call(r)
					if commandErr, ok := err.(*Z3ReplayCommandError); ok {
						_, writeErr := fmt.Fprintln(r.output, "[z3 exception]: "+commandErr.Message)
						err = writeErr
					}
				}
			}
		case '=':
			var p uint64
			p, err = s.hexadecimal()
			if err == nil {
				r.saveResult(p)
			}
		case '*':
			var p, pos uint64
			p, err = s.hexadecimal()
			if err == nil {
				s.blanks()
				pos, err = s.uint64()
			}
			if err == nil {
				err = r.saveArgument(p, pos)
			}
		case '@':
			var p, pos, idx uint64
			p, err = s.hexadecimal()
			if err == nil {
				s.blanks()
				pos, err = s.uint64()
			}
			if err == nil {
				s.blanks()
				idx, err = s.uint64()
			}
			if err == nil {
				err = r.saveArrayElement(p, pos, idx)
			}
		case 'M':
			var message string
			message, err = s.quoted('"')
			if err == nil {
				_, err = fmt.Fprintln(r.output, message)
			}
		default:
			err = fmt.Errorf("unknown log command")
		}
		if err != nil {
			if _, ok := err.(*Z3ReplayError); ok {
				return err
			}
			return r.failure(err.Error())
		}
	}
}
