package utils

import (
	"hack-browser-data/log"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

const (
	LoginData = "Login Data"
	History   = "History"
	Cookies   = "Cookies"
	WebData   = "Web Data"
	Bookmarks = "Bookmarks"
)

func CopyDB(src, dst string) error {
	locals, _ := filepath.Glob("*")
	for _, v := range locals {
		if v == dst {
			err := os.Remove(dst)
			if err != nil {
				return err
			}
		}
	}
	sourceFile, err := ioutil.ReadFile(src)
	if err != nil {
		log.Println(err.Error())
	}

	err = ioutil.WriteFile(dst, sourceFile, 0777)
	if err != nil {
		log.Println(err.Error())
	}
	return err
}

func ParseBookMarks() {

}

func RemoveFile() {

}

func TimeEpochFormat(epoch int64) time.Time {
	t := time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)
	d := time.Duration(epoch)
	for i := 0; i < 1000; i++ {
		t = t.Add(d)
	}
	return t
}

func ReadFile(filename string) (string, error) {
	s, err := ioutil.ReadFile(filename)
	return string(s), err
}

//func MakeDir(dirName string) error {
//
//}
