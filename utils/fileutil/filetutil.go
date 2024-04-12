package fileutil

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path"
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
		return err
	}
	b := new(bytes.Buffer)
	zw := zip.NewWriter(b)
	for _, f := range files {
		fw, err := zw.Create(f.Name())
		if err != nil {
			return err
		}
		name := path.Join(dir, f.Name())
		content, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		_, err = fw.Write(content)
		if err != nil {
			return err
		}
		err = os.Remove(name)
		if err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	filename := filepath.Join(dir, fmt.Sprintf("%s.zip", dir))
	outFile, err := os.Create(filepath.Clean(filename))
	if err != nil {
		return err
	}
	_, err = b.WriteTo(outFile)
	if err != nil {
		return err
	}
	return outFile.Close()
}
