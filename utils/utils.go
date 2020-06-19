package utils

import (
	"hack-browser-data/log"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	LoginData = "Login Data"
	History   = "History"
	Cookies   = "Cookies"
	WebData   = "Web Data"
)

func CopyDB(source, dest string) error {
	// remove current path db file first
	locals, _ := filepath.Glob("*")
	for _, v := range locals {
		if v == dest {
			err := os.Remove(dest)
			if err != nil {
				return err
			}
		}
	}
	sourceFile, err := ioutil.ReadFile(source)
	if err != nil {
		log.Println(err.Error())
	}

	err = ioutil.WriteFile(dest, sourceFile, 644)
	if err != nil {
		log.Println(err.Error())
	}
	err = os.Chmod(dest, 0777)
	return err
}
