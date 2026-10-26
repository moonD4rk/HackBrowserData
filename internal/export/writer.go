package export

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

type Writer struct {
	dir    string
	format Format
	files  map[category.Category]*categoryFile
	failed bool
}

type categoryFile struct {
	name  string
	file  *os.File
	buf   *bufio.Writer
	enc   encoder
	count int
}

func NewWriter(dir string, f Format) (*Writer, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	return &Writer{dir: dir, format: f, files: make(map[category.Category]*categoryFile)}, nil
}

// Write reorders records in place.
func (w *Writer) Write(c category.Category, records []record.Record) error {
	if err := w.write(c, records); err != nil {
		w.failed = true
		return err
	}
	return nil
}

func (w *Writer) write(c category.Category, records []record.Record) error {
	if len(records) == 0 {
		return nil
	}
	for _, r := range records {
		if r.Category() != c {
			return fmt.Errorf("%T is not a %s record", r, c)
		}
	}
	cf, err := w.open(c)
	if err != nil {
		return err
	}
	sortRecords(records)
	for _, r := range records {
		if err := cf.enc.encode(r); err != nil {
			return fmt.Errorf("write %s: %w", cf.name, err)
		}
		cf.count++
	}
	return nil
}

// Close finishes the files even after a failed Write, so they keep what was written; only a clean run lists them.
func (w *Writer) Close() error {
	var errs []error
	for _, c := range category.All() {
		if cf, ok := w.files[c]; ok {
			errs = append(errs, cf.close())
		}
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	if w.failed || len(w.files) == 0 {
		return nil
	}
	fmt.Fprintln(os.Stderr)
	log.Infof("Exported to %s/", w.dir)
	for _, c := range category.All() {
		if cf, ok := w.files[c]; ok {
			log.Infof("  %-24s %d entries", cf.name, cf.count)
		}
	}
	return nil
}

func (w *Writer) open(c category.Category) (*categoryFile, error) {
	if cf, ok := w.files[c]; ok {
		return cf, nil
	}
	name := c.String() + "." + w.format.ext()
	f, err := os.OpenFile(filepath.Join(w.dir, name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", name, err)
	}
	buf := bufio.NewWriter(f)
	cf := &categoryFile{name: name, file: f, buf: buf, enc: newEncoder(buf, w.format, c)}
	w.files[c] = cf
	return cf, nil
}

func (cf *categoryFile) close() error {
	err := cf.enc.end()
	if err == nil {
		err = cf.buf.Flush()
	}
	if err = errors.Join(err, cf.file.Close()); err != nil {
		return fmt.Errorf("write %s: %w", cf.name, err)
	}
	return nil
}
