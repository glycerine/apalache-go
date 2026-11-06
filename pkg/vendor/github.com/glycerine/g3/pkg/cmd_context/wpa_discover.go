package cmd_context

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glycerine/g3/pkg/muz/base"
)

func (p *wpaParser) files(extension string) ([]string, error) {
	var result []string
	err := filepath.WalkDir(p.directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && path != p.directory && strings.HasPrefix(entry.Name(), ".") {
			return filepath.SkipDir
		}
		if !entry.IsDir() && filepath.Ext(path) == "."+extension {
			result = append(result, path)
		}
		return nil
	})
	sort.Strings(result)
	return result, err
}

func wpaBase(path string) string {
	return base.FileNameWithoutExtension(path)
}
