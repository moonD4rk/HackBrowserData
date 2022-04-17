package fileutil

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"hack-browser-data/internal/item"
	"hack-browser-data/internal/log"
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

// CopyItemToLocal copies the file from the provided path to the local path
func CopyItemToLocal(itemPaths map[item.Item]string) error {
	for i, p := range itemPaths {
		// var dstFilename = item.TempName()
		var filename = i.String()
		// TODO: Handle read file error
		d, err := ioutil.ReadFile(p)
		if err != nil {
			fmt.Println(err.Error())
		}
		err = ioutil.WriteFile(filename, d, 0777)
		if err != nil {
			return err
		}
	}
	return nil
}

func ParentDir(p string) string {
	return filepath.Dir(p)
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
	filename := filepath.Join(dir, "archive.zip")
	outFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	_, err = b.WriteTo(outFile)
	if err != nil {
		return err
	}
	log.Debugf("Compress success, zip filename is %s", filename)
	return nil
}
