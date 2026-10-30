package export

import (
	"archive/zip"
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
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
	name  string
	file  *os.File
	buf   *bufio.Writer
	enc   encoder
	count int
}

// NewWriter creates dir. With compress, Close packs the files it wrote into <dir>/<base of dir>.zip.
func NewWriter(dir string, f Format, compress bool) (*Writer, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	return &Writer{dir: dir, format: f, compress: compress, files: make(map[category.Category]*categoryFile)}, nil
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

// Close finishes the files even after a failed Write, so they keep what was written; only a clean run lists them in Output.
func (w *Writer) Close() error {
	var (
		errs  []error
		files []File
	)
	for _, c := range category.All() {
		if cf, ok := w.files[c]; ok {
			errs = append(errs, cf.close())
			files = append(files, File{Name: cf.name, Count: cf.count})
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
	for _, cf := range w.files {
		names = append(names, cf.name)
	}
	sort.Strings(names)
	zipName := filepath.Base(w.dir) + ".zip"
	if err := writeZip(filepath.Join(w.dir, zipName), w.dir, names); err != nil {
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

// writeZip writes the zip under a temp name beside dst and renames it into place once it is complete.
func writeZip(dst, dir string, names []string) (err error) {
	tmp, err := os.OpenFile(dst+".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.Remove(tmp.Name()))
		}
	}()
	zw := zip.NewWriter(tmp)
	for _, name := range names {
		if err := addZipFile(zw, filepath.Join(dir, name), name); err != nil {
			return errors.Join(fmt.Errorf("add %s: %w", name, err), zw.Close(), tmp.Close())
		}
	}
	if err := zw.Close(); err != nil {
		return errors.Join(fmt.Errorf("finish %s: %w", dst, err), tmp.Close())
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("finish %s: %w", dst, err)
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return fmt.Errorf("move zip into place: %w", err)
	}
	return nil
}

func addZipFile(zw *zip.Writer, path, name string) error {
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	return err
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
