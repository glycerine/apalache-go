package lp

// Source: src/test/lp.cpp:1772-1780,2508-2527,3149-3154.

import (
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// ReadDriverFileList reads literal filenames until the first empty line or EOF.
// Spaces, carriage returns, duplicates and relative paths remain unchanged.
// A missing file reports the source diagnostic and returns an empty list.
// Native read and report failures return the completed prefix and an error.
func ReadDriverFileList(name string, out io.Writer) ([]string, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, writeDriverLine(out, "cannot open "+name)
	}
	defer f.Close()
	return readDriverFileList(f)
}

func readDriverFileList(input io.Reader) ([]string, error) {
	r := bufio.NewReader(input)
	var names []string
	for {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			return names, err
		}
		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			return names, nil
		}
		names = append(names, line)
		if err == io.EOF {
			return names, nil
		}
	}
}

// MPSFileListDriver implements the retained --filelist solve loop. It borrows
// arguments and the numerical manager. Each completed solver closes before the
// next starts; a failed solver remains Current until Close succeeds.
type MPSFileListDriver struct {
	Name      string
	Arguments *DriverArguments
	Files     []string
	Completed int
	Current   *MPSFileDriver
	manager   *numeral.RationalManager
	config    MPSDriverConfig
	started   bool
	closed    bool
}

func NewMPSFileListDriver(name string, args *DriverArguments, manager *numeral.RationalManager, config MPSDriverConfig) *MPSFileListDriver {
	if config.Output == nil {
		config.Output = os.Stdout
	}
	return &MPSFileListDriver{Name: name, Arguments: args, manager: manager, config: config}
}

func (d *MPSFileListDriver) Run() error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	var err error
	d.Files, err = ReadDriverFileList(d.Name, d.config.Output)
	if err != nil {
		return err
	}
	for _, name := range d.Files {
		d.Current = NewMPSFileDriver(name, d.Arguments, d.manager, d.config)
		if err = d.Current.Run(); err != nil {
			return err
		}
		d.Current = nil
		d.Completed++
	}
	return d.Close()
}

func (d *MPSFileListDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.Current != nil {
		if err := d.Current.Close(); err != nil {
			return err
		}
		d.Current = nil
	}
	d.closed = true
	return nil
}
