package cmd_context

import (
	"bufio"
	"errors"
	"fmt"
	"os"
)

func joinClose(err error, closeErr error) error { return errors.Join(err, closeErr) }

func (p *wpaParser) readMaps() error {
	files, err := p.files("map")
	if err != nil {
		return err
	}
	for _, path := range files {
		name := wpaBase(path)
		entry := p.sorts[name]
		if entry == nil {
			entry = &wpaSort{name: name, numbers: make(map[uint64]bool)}
			p.sorts[name] = entry
		}
		file, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 1024), int(^uint(0)>>1))
		line := 0
		for scanner.Scan() {
			line++
			number, name, present, parseErr := parseWPAMapLineWithNames(scanner.Text(), p.useMapNames)
			if parseErr != nil {
				file.Close()
				return fmt.Errorf("%s:%d: %w", path, line, parseErr)
			}
			if !present {
				continue
			}
			entry.numbers[number] = true
			// Source num2sym::insert_if_not_there2 keeps the first name when
			// independent sort maps reuse a numeric symbol. A different later
			// spelling is warning-only and must not reject the directory.
			if p.useMapNames {
				if _, found := p.mapNames[number]; !found {
					p.mapNames[number] = name
				}
			}
		}
		err = scanner.Err()
		err = joinClose(err, file.Close())
		if err != nil {
			return err
		}
	}
	return nil
}
