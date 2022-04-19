package fileutil

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"hack-browser-data/internal/log"

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

// ReadFile reads the file from the provided path
func ReadFile(filename string) (string, error) {
	s, err := ioutil.ReadFile(filename)
	return string(s), err
}

func CopyDir(src, dst, skip string) error {
	s := cp.Options{Skip: func(src string) (bool, error) {
		return strings.Contains(strings.ToLower(src), skip), nil
	}}
	return cp.Copy(src, dst, s)
}

func Filename(browser, item, ext string) string {
	replace := strings.NewReplacer(" ", "_", ".", "_", "-", "_")
	return strings.ToLower(fmt.Sprintf("%s_%s.%s", replace.Replace(browser), item, ext))
}

func ParentDir(p string) string {
	return filepath.Dir(filepath.Clean(p))
}

func BaseDir(p string) string {
	return filepath.Base(p)
}

func ParentBaseDir(p string) string {
	return BaseDir(ParentDir(p))
}

func CompressDir(dir string) error {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Error(err)
	}
	var b = new(bytes.Buffer)
	zw := zip.NewWriter(b)
	for _, f := range files {
		fw, _ := zw.Create(f.Name())
		fileName := path.Join(dir, f.Name())
		fileContent, err := ioutil.ReadFile(fileName)
		if err != nil {
			zw.Close()
			return err
		}
		_, err = fw.Write(fileContent)
		if err != nil {
			zw.Close()
			return err
		}
		err = os.Remove(fileName)
		if err != nil {
			log.Error(err)
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	filename := filepath.Join(dir, fmt.Sprintf("%s.zip", dir))
	outFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	_, err = b.WriteTo(outFile)
	if err != nil {
		return err
	}
	log.Noticef("compress success, zip filename is %s", filename)
	return nil
}
