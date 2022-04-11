package fileutil

import (
	"fmt"
	"io/ioutil"
	"os"

	"hack-browser-data/internal/item"
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
	for i, path := range itemPaths {
		// var dstFilename = item.TempName()
		var filename = i.String()
		// TODO: Handle read file error
		d, err := ioutil.ReadFile(path)
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
