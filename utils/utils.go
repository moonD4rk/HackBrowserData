package utils

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"hack-browser-data/log"
)

const Prefix = "[x]: "

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
		log.Debug(err.Error())
	}
	err = ioutil.WriteFile(dst, sourceFile, 0777)
	if err != nil {
		log.Debug(err.Error())
	}
	return err
}

func GetItemPath(profilePath, file string) (string, error) {
	p, err := filepath.Glob(profilePath + file)
	if err != nil {
		return "", err
	}
	if len(p) > 0 {
		return p[0], nil
	}
	return "", fmt.Errorf("find %s failed", file)
}

func IntToBool(a int) bool {
	switch a {
	case 0, -1:
		return false
	}
	return true
}

func BookMarkType(a int64) string {
	switch a {
	case 1:
		return "url"
	default:
		return "folder"
	}
}

func TimeStampFormat(stamp int64) time.Time {
	s1 := time.Unix(stamp, 0)
	return s1
}

func TimeEpochFormat(epoch int64) time.Time {
	maxTime := int64(99633311740000000)
	if epoch > maxTime {
		return time.Date(2049, 1, 1, 1, 1, 1, 1, time.Local)
	}
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

func WriteFile(filename string, data []byte) error {
	err := ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		return nil
	}
	return err
}

func FormatFileName(dir, browser, filename, format string) string {
	r := strings.Replace(strings.TrimSpace(strings.ToLower(browser)), " ", "_", -1)
	p := path.Join(dir, fmt.Sprintf("%s_%s.%s", r, filename, format))
	return p
}

func MakeDir(dirName string) error {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		return os.Mkdir(dirName, 0700)
	}
	return nil
}

func Compress(exportDir string) error {
	files, err := ioutil.ReadDir(exportDir)
	if err != nil {
		log.Error(err)
	}
	var b = new(bytes.Buffer)
	zw := zip.NewWriter(b)
	for _, f := range files {
		fw, _ := zw.Create(f.Name())
		fileName := path.Join(exportDir, f.Name())
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
	zipName := exportDir + `/archive.zip`
	outFile, _ := os.Create(zipName)
	_, err = b.WriteTo(outFile)
	if err != nil {
		return err
	}
	fmt.Printf("%s Compress success, zip filename is %s \n", Prefix, zipName)
	return nil
}
