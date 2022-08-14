package fileutil

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	cp "github.com/otiai10/copy"
)

// FileExists checks if the file exists in the provided path
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// FolderExists checks if the folder exists
func FolderExists(foldername string) bool {
	info, err := os.Stat(foldername)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return false
	}
	return info.IsDir()
}

// FilesInFolder returns the filepath contains in the provided folder
func FilesInFolder(dir, filename string) ([]string, error) {
	if !FolderExists(dir) {
		return nil, errors.New(dir + " folder does not exist")
	}
	var files []string
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() && strings.HasSuffix(path, filename) {
			files = append(files, path)
		}
		return err
	})
	return files, err
}

// ReadFile reads the file from the provided path
func ReadFile(filename string) (string, error) {
	s, err := os.ReadFile(filename)
	return string(s), err
}

// CopyDir copies the directory from the source to the destination
// skip the file if you don't want to copy
func CopyDir(src, dst, skip string) error {
	s := cp.Options{Skip: func(src string) (bool, error) {
		return strings.HasSuffix(strings.ToLower(src), skip), nil
	}}
	return cp.Copy(src, dst, s)
}

// CopyDirHasSuffix copies the directory from the source to the destination
// contain is the file if you want to copy, and rename copied filename with dir/index_filename
func CopyDirHasSuffix(src, dst, suffix string) error {
	var files []string
	err := filepath.Walk(src, func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), suffix) {
			files = append(files, path)
		}
		return err
	})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	for index, file := range files {
		// p = dir/index_file
		p := fmt.Sprintf("%s/%d_%s", dst, index, BaseDir(file))
		err = CopyFile(file, p)
		if err != nil {
			return err
		}
	}
	return nil
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

// ItemName returns the filename from the provided path
func ItemName(browser, item, ext string) string {
	replace := strings.NewReplacer(" ", "_", ".", "_", "-", "_")
	return strings.ToLower(fmt.Sprintf("%s_%s.%s", replace.Replace(browser), item, ext))
}

func BrowserName(browser, user string) string {
	replace := strings.NewReplacer(" ", "_", ".", "_", "-", "_", "Profile", "User")
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
