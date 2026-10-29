package escaped

// Source: src/util/util.h:379-389; src/util/util.cpp:122-152.

import (
	"errors"
	"io"
	"strings"
)

var ErrTooLarge = errors.New("escaped: output is too large")

const maxOutputBytes = uint64(1<<31 - 1)

func prefix(text *string, trim bool) string {
	if text == nil {
		return ""
	}
	s := *text
	if end := strings.IndexByte(s, 0); end >= 0 {
		s = s[:end]
	}
	if trim {
		s = strings.TrimRight(s, "\n")
	}
	return s
}

func outputSize(s string, indent uint32) (int, error) {
	size := uint64(len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			size++
		case '\n':
			size += uint64(indent)
		}
		if size > maxOutputBytes {
			return 0, ErrTooLarge
		}
	}
	return int(size), nil
}

func writeString(w io.Writer, s string) error {
	n, err := io.WriteString(w, s)
	if err == nil && n != len(s) {
		return io.ErrShortWrite
	}
	return err
}

func writeSpaces(w io.Writer, count uint32) error {
	const spaces = "                                                                "
	for count != 0 {
		n := count
		if n > uint32(len(spaces)) {
			n = uint32(len(spaces))
		}
		if err := writeString(w, spaces[:n]); err != nil {
			return err
		}
		count -= n
	}
	return nil
}

// Write emits source escaped formatting to w. It escapes double quotes and
// emits indent spaces after each retained newline.
func Write(w io.Writer, text *string, trim bool, indent uint32) error {
	if w == nil {
		return io.ErrClosedPipe
	}
	s := prefix(text, trim)
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			if err := writeString(w, "\\"); err != nil {
				return err
			}
		}
		if err := writeString(w, s[i:i+1]); err != nil {
			return err
		}
		if s[i] == '\n' && indent != 0 {
			if err := writeSpaces(w, indent); err != nil {
				return err
			}
		}
	}
	return nil
}

func String(text *string, trim bool, indent uint32) (string, error) {
	s := prefix(text, trim)
	size, err := outputSize(s, indent)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.Grow(size)
	if err = Write(&out, &s, false, indent); err != nil {
		return "", err
	}
	return out.String(), nil
}
