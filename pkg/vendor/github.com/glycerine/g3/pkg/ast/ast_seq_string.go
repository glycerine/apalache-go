package ast

// Source: src/ast/seq_decl_plugin.cpp:20-142,844-858,948-956;
// src/ast/seq_decl_plugin.h:35-37,256-263.

import (
	"math"
	"strconv"
	"strings"
)

const (
	ASTSequenceOpUnit        int32 = 0
	ASTSequenceOpEmpty       int32 = 1
	ASTSequenceOpConcat      int32 = 2
	ASTSequenceOpToRegex     int32 = 11
	ASTSequenceOpStringConst int32 = 26
)

func encodeSequenceString(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c < 32:
			escapes := [...]string{"\\x00", "\\x01", "\\x02", "\\x03", "\\x04", "\\x05", "\\x06", "\\x07", "\\x08", "\\x09", "\\n", "\\v", "\\f", "\\r", "\\x0E", "\\x0F", "\\x10", "\\x11", "\\x12", "\\x13", "\\x14", "\\x15", "\\x16", "\\x17", "\\x18", "\\x19", "\\x1A", "\\x1B", "\\x1C", "\\x1D", "\\x1E", "\\x1F"}
			out.WriteString(escapes[c])
		case c == '\\':
			out.WriteString("\\\\")
		case c >= 128:
			out.WriteString("\\x")
			out.WriteString(strconv.FormatUint(uint64(c), 16))
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

func sequenceHexDigit(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	}
	return 0, false
}

func decodeSequenceString(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		if value[i] != '\\' || i+1 == len(value) {
			out.WriteByte(value[i])
			i++
			continue
		}
		if i+3 < len(value) && value[i+1] == 'x' {
			hi, ok1 := sequenceHexDigit(value[i+2])
			lo, ok2 := sequenceHexDigit(value[i+3])
			if ok1 && ok2 {
				out.WriteByte(hi<<4 | lo)
				i += 4
				continue
			}
		}
		j := i + 1
		var octal byte
		for n := 0; n < 3 && j < len(value) && value[j] >= '0' && value[j] <= '7'; n++ {
			octal = octal*8 + value[j] - '0'
			j++
		}
		if j != i+1 {
			out.WriteByte(octal)
			i = j
			continue
		}
		switch value[i+1] {
		case 'a':
			out.WriteByte('\a')
		case 'b':
			out.WriteByte('\b')
		case 'f':
			out.WriteByte('\f')
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case 'v':
			out.WriteByte('\v')
		default:
			out.WriteByte(value[i+1])
		}
		i += 2
	}
	return out.String()
}

// MkString constructs the retained seq_decl_plugin string constant.
func (m *ASTManager) prepareSequencePlugin() error {
	if _, err := m.RegisterArithmeticPlugin(); err != nil {
		return err
	}
	if _, err := m.RegisterBitVectorPlugin(); err != nil {
		return err
	}
	if _, err := m.RegisterArrayPlugin(); err != nil {
		return err
	}
	if _, err := m.RegisterSequencePlugin(); err != nil {
		return err
	}
	return nil
}

func (m *ASTManager) MkString(value string) (ASTHandle, error) {
	if err := m.prepareSequencePlugin(); err != nil {
		return ASTHandle{}, err
	}
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	app, err := m.mkSequenceString(value)
	return m.external(app), err
}

func (m *ASTManager) mkSequenceString(value string) (astNodeRef, error) {
	p := m.sequencePlugin
	if p == nil {
		return astNodeRef{}, ErrASTHandle
	}
	symbol, err := m.Symbols.Intern(encodeSequenceString(value))
	if err != nil {
		return astNodeRef{}, err
	}
	parameter := astParameterRecord{Kind: ASTParameterSymbol, symbol: symbol}
	if err = m.beginFormatInfo(false, p.family, ASTSequenceOpStringConst, &parameter); err != nil {
		return astNodeRef{}, err
	}
	decl, err := m.mkFuncDecl(p.names[0], nil, m.external(p.stringSort), ASTFuncDeclInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
	m.endFormatInfo()
	if err != nil {
		return astNodeRef{}, err
	}
	return m.mkApp(decl, nil)
}

// SequenceStringValue recognizes a string constant and returns its decoded
// byte string.
func (m *ASTManager) SequenceStringValue(h ASTHandle) (value string, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return "", false, err
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return "", false, nil
	}
	p := m.sequencePlugin
	if p == nil {
		return "", false, nil
	}
	decl := m.appAt(r.slot).decl
	family, kind, err := m.declIdentity(decl)
	if err != nil || family != p.family || kind != ASTSequenceOpStringConst {
		return "", false, err
	}
	info, err := m.funcInfo(m.declAt(decl.slot).info)
	if err != nil || info.count != 1 {
		return "", false, err
	}
	token, err := m.infoParameter(info, 0)
	if err != nil {
		return "", false, err
	}
	parameter, err := m.parameter(token)
	if err != nil || parameter.Kind != ASTParameterSymbol {
		if err == nil {
			err = ErrASTDomain
		}
		return "", false, err
	}
	encoded, err := m.Symbols.String(parameter.symbol)
	if err != nil {
		return "", false, err
	}
	return decodeSequenceString(encoded), true, nil
}
