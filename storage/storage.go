package storage

import (
	"errors"
	"io"
)

type File interface {
	io.WriterAt
	io.Closer
}

type Provider interface {
	Open(path string) (File, error)
}

type MultiProvider []Provider

func (x MultiProvider) Open(path string) (File, error) {
	var files multiFile
	var err error

	for _, provider := range x {
		f, pe := provider.Open(path)
		files = append(files, f)
		err = errors.Join(err, pe)
	}

	return files, err
}

type multiFile []File

func (x multiFile) WriteAt(p []byte, off int64) (int, error) {
	var err error
	var count int

	for _, file := range x {
		n, we := file.WriteAt(p, off)
		err = errors.Join(err, we)
		count = n
	}

	return count, err
}

func (x multiFile) Close() error {
	var err error
	for _, file := range x {
		err = errors.Join(err, file.Close())
	}

	return err
}
