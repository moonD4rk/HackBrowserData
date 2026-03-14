package profile

import (
	"io/fs"
	"path/filepath"
)

type FileSystem interface {
	WalkDir(root string, fn fs.WalkDirFunc) error
}

type osFS struct{}

func (o osFS) WalkDir(root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, fn)
}
