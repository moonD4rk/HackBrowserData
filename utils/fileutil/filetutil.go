package fileutil

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cp "github.com/otiai10/copy"
)

// IsFileExists checks if the file exists in the provided path
func IsFileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// IsDirExists checks if the folder exists
func IsDirExists(folder string) bool {
	info, err := os.Stat(folder)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ReadFile reads the file from the provided path
func ReadFile(filename string) (string, error) {
	s, err := os.ReadFile(filename)
	return string(s), err
}

// CopyDir copies the directory from the source to the destination
// skip the file if you don't want to copy
func CopyDir(src, dst, skip string) error {
	s := cp.Options{Skip: func(info os.FileInfo, src, dst string) (bool, error) {
		return strings.HasSuffix(strings.ToLower(src), skip), nil
	}}
	return cp.Copy(src, dst, s)
}

// CopyFile copies the file from the source to the destination
func CopyFile(src, dst string) error {
	s, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	err = os.WriteFile(dst, s, 0o600)
	if err != nil {
		return err
	}
	return nil
}

// Filename returns the filename from the provided path
func Filename(browser, dataType, ext string) string {
	replace := strings.NewReplacer(" ", "_", ".", "_", "-", "_")
	return strings.ToLower(fmt.Sprintf("%s_%s.%s", replace.Replace(browser), dataType, ext))
}

func BrowserName(browser, user string) string {
	replace := strings.NewReplacer(" ", "_", ".", "_", "-", "_", "Profile", "user")
	return strings.ToLower(fmt.Sprintf("%s_%s", replace.Replace(browser), replace.Replace(user)))
}

// ParentDir returns the parent directory of the provided path
func ParentDir(p string) string {
	return filepath.Dir(filepath.Clean(p))
}

// BaseDir returns the base directory of the provided path
func BaseDir(p string) string {
	return filepath.Base(p)
}

// ParentBaseDir returns the parent base directory of the provided path
func ParentBaseDir(p string) string {
	return BaseDir(ParentDir(p))
}

// CompressDir compresses the directory into a zip file
func CompressDir(dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir error: %w", err)
	}
	if len(files) == 0 {
		// Return an error if no files are found in the directory
		return fmt.Errorf("no files to compress in: %s", dir)
	}

	buffer := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buffer)
	defer func() {
		_ = zipWriter.Close()
	}()

	for _, file := range files {
		if err := addFileToZip(zipWriter, filepath.Join(dir, file.Name())); err != nil {
			return fmt.Errorf("failed to add file to zip: %w", err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("error closing zip writer: %w", err)
	}

	zipFilename := filepath.Join(dir, filepath.Base(dir)+".zip")
	return writeFile(buffer, zipFilename)
}

func addFileToZip(zw *zip.Writer, filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading file %s: %w", filename, err)
	}

	fw, err := zw.Create(filepath.Base(filename))
	if err != nil {
		return fmt.Errorf("error creating zip entry for %s: %w", filename, err)
	}

	if _, err = fw.Write(content); err != nil {
		return fmt.Errorf("error writing content to zip for %s: %w", filename, err)
	}

	if err = os.Remove(filename); err != nil {
		return fmt.Errorf("error removing original file %s: %w", filename, err)
	}

	return nil
}

func writeFile(buffer *bytes.Buffer, filename string) error {
	outFile, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating output file %s: %w", filename, err)
	}
	defer func() {
		_ = outFile.Close()
	}()

	if _, err = buffer.WriteTo(outFile); err != nil {
		return fmt.Errorf("error writing data to file %s: %w", filename, err)
	}

	return nil
}
