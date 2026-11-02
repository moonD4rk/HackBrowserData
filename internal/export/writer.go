package export

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/zipfile"
)

type Writer struct {
	dir      string
	format   Format
	compress bool
	files    map[category.Category]*categoryFile
	failed   bool
	out      Output
}

// Output is what a clean Close left in the output dir: each category file in category order, and the zip with --zip.
type Output struct {
	Dir   string
	Files []File
	Zip   string
}

type File struct {
	Name  string
	Count int // records written
}

type categoryFile struct {
	name     string
	file     *os.File
	buffered *bufio.Writer
	encoder  encoder
	count    int
}

// NewWriter creates dir. With compress, Close packs the files it wrote into <dir>/<base of dir>.zip.
func NewWriter(dir string, format Format, compress bool) (*Writer, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	return &Writer{dir: dir, format: format, compress: compress, files: make(map[category.Category]*categoryFile)}, nil
}

// Write reorders records in place.
func (w *Writer) Write(dataCategory category.Category, records []record.Record) error {
	if err := w.write(dataCategory, records); err != nil {
		w.failed = true
		return err
	}
	return nil
}

func (w *Writer) write(dataCategory category.Category, records []record.Record) error {
	if len(records) == 0 {
		return nil
	}
	for _, row := range records {
		if row.Category() != dataCategory {
			return fmt.Errorf("%T is not a %s record", row, dataCategory)
		}
	}
	file, err := w.open(dataCategory)
	if err != nil {
		return err
	}
	sortRecords(records)
	for _, row := range records {
		if err := file.encoder.encode(row); err != nil {
			return fmt.Errorf("write %s: %w", file.name, err)
		}
		file.count++
	}
	return nil
}

// Close completes the files even after a failed Write, so they keep what was written; only a clean run fills Output.
func (w *Writer) Close() error {
	var (
		errs  []error
		files []File
	)
	for _, dataCategory := range category.All() {
		if file, ok := w.files[dataCategory]; ok {
			errs = append(errs, file.close())
			files = append(files, File{Name: file.name, Count: file.count})
		}
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	if w.failed {
		return nil
	}
	if len(w.files) == 0 {
		if w.compress {
			return fmt.Errorf("compress: no files to compress in %s", w.dir)
		}
		return nil
	}
	w.out = Output{Dir: w.dir, Files: files}
	if w.compress {
		return w.pack()
	}
	return nil
}

func (w *Writer) Output() Output {
	return w.out
}

// pack removes the loose files only after the zip is in place, so a failure leaves every one of them where it was.
func (w *Writer) pack() error {
	names := make([]string, 0, len(w.files))
	for _, file := range w.files {
		names = append(names, file.name)
	}
	sort.Strings(names)
	zipName := filepath.Base(w.dir) + ".zip"
	if err := zipfile.Write(filepath.Join(w.dir, zipName), w.dir, names); err != nil {
		return fmt.Errorf("compress: %w", err)
	}
	for _, name := range names {
		if err := os.Remove(filepath.Join(w.dir, name)); err != nil {
			return fmt.Errorf("compress: remove %s: %w", name, err)
		}
	}
	w.out.Zip = zipName
	return nil
}

func (w *Writer) open(dataCategory category.Category) (*categoryFile, error) {
	if existing, ok := w.files[dataCategory]; ok {
		return existing, nil
	}
	name := dataCategory.String() + "." + w.format.ext()
	file, err := os.OpenFile(filepath.Join(w.dir, name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", name, err)
	}
	buffered := bufio.NewWriter(file)
	opened := &categoryFile{name: name, file: file, buffered: buffered, encoder: newEncoder(buffered, w.format, dataCategory)}
	w.files[dataCategory] = opened
	return opened, nil
}

func (cf *categoryFile) close() error {
	err := cf.encoder.end()
	if err == nil {
		err = cf.buffered.Flush()
	}
	if err = errors.Join(err, cf.file.Close()); err != nil {
		return fmt.Errorf("write %s: %w", cf.name, err)
	}
	return nil
}
