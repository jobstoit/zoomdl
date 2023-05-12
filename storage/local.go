package storage

import (
	"os"
	"path/filepath"
)

type Local struct {
	Path string
}

func NewLocal(path string) Provider {
	return &Local{
		Path: path,
	}
}

func (x Local) Open(path string) (File, error) {
	return os.OpenFile(filepath.Join(x.Path, path), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
}
