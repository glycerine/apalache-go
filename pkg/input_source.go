package apalache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type InputFormat string

const (
	InputFormatTLA   InputFormat = "tla"
	InputFormatJSON  InputFormat = "json"
	InputFormatITF   InputFormat = "itf"
	InputFormatQuint InputFormat = "qnt"
)

func ParseInputFormat(value string) (InputFormat, error) {
	switch InputFormat(strings.ToLower(value)) {
	case InputFormatTLA, InputFormatJSON, InputFormatITF, InputFormatQuint:
		return InputFormat(strings.ToLower(value)), nil
	default:
		return "", fmt.Errorf("unsupported source format: %s", strings.ToLower(value))
	}
}

type InputSource interface {
	Format() InputFormat
	Exists() bool
	ReadUTF8() (string, error)
	fmt.Stringer
}

type FileInputSource struct {
	Path   string
	format InputFormat
}

func NewFileInputSource(path string) (FileInputSource, error) {
	filename := strings.ToLower(filepath.Base(path))
	var format InputFormat
	switch {
	case strings.HasSuffix(filename, ".itf.json"):
		format = InputFormatITF
	case strings.HasSuffix(filename, ".qnt.json"):
		format = InputFormatQuint
	case strings.HasSuffix(filename, ".json"):
		format = InputFormatJSON
	case strings.HasSuffix(filename, ".tla"):
		format = InputFormatTLA
	default:
		extension := strings.TrimPrefix(filepath.Ext(filename), ".")
		return FileInputSource{}, fmt.Errorf("unsupported file format: %s", extension)
	}
	return FileInputSource{Path: path, format: format}, nil
}

func NewFileInputSourceWithFormat(path string, format InputFormat) FileInputSource {
	return FileInputSource{Path: path, format: format}
}

func (source FileInputSource) Format() InputFormat { return source.format }
func (source FileInputSource) String() string      { return source.Path }
func (source FileInputSource) Exists() bool {
	_, err := os.Stat(source.Path)
	return err == nil
}
func (source FileInputSource) ReadUTF8() (string, error) {
	data, err := os.ReadFile(source.Path)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("file not found: %s", source.Path)
	}
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w", source, err)
	}
	return string(data), nil
}

type StringInputSource struct {
	Content   string
	Auxiliary []string
	format    InputFormat
}

func NewStringInputSource(content string, auxiliary []string, format InputFormat) StringInputSource {
	return StringInputSource{Content: content, Auxiliary: append([]string(nil), auxiliary...), format: format}
}

func (source StringInputSource) Format() InputFormat       { return source.format }
func (source StringInputSource) Exists() bool              { return true }
func (source StringInputSource) ReadUTF8() (string, error) { return source.Content, nil }
func (source StringInputSource) String() string {
	return fmt.Sprintf("StringSource(%s)", source.format)
}
