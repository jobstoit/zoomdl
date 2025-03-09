package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"

	"github.com/jobstoit/s3io/v3"
)

type FileSystem interface {
	Writer(ctx context.Context, target string) (io.WriteCloser, error)
	Reader(ctx context.Context, target string) (io.Reader, error)
}

type multifs []FileSystem

func newMultiFS(ctx context.Context, destinations []string) (FileSystem, error) {
	fileSystems := make(multifs, 0, len(destinations))

	for _, dst := range destinations {
		if dst == "" {
			continue
		}

		u, err := url.Parse(dst)
		if err != nil {
			return nil, err
		}

		switch u.Scheme {
		case "file":
			fs, err := newOsFS(u.Path)
			if err != nil {
				return nil, fmt.Errorf("unable to open '%s': %v", u.Path, err)
			}

			fileSystems = append(fileSystems, fs)
		case "s3":
			bucket, err := s3io.OpenURL(ctx, dst)
			if err != nil {
				return nil, err
			}

			fileSystems = append(fileSystems, &s3fs{bucket: bucket})
		default:
			return nil, fmt.Errorf("unrecognised destination '%s'", dst)
		}
	}

	return fileSystems, nil
}

func (f multifs) Writer(ctx context.Context, target string) (io.WriteCloser, error) {
	writers := multiWriteCloser{}

	for _, t := range f {
		file, err := t.Writer(ctx, target)
		if err != nil {
			defer writers.Close()

			return nil, err
		}

		writers = append(writers, file)
	}

	return writers, nil
}

func (f multifs) Reader(ctx context.Context, target string) (io.Reader, error) {
	if len(f) < 1 {
		return nil, fmt.Errorf("no fs available")
	}

	return f[0].Reader(ctx, target)
}

type osfs struct {
	base string
}

func newOsFS(base string) (*osfs, error) {
	err := os.MkdirAll(base, os.ModePerm)
	if err != nil {
		return nil, err
	}

	return &osfs{
		base: base,
	}, nil
}

func (f *osfs) Writer(_ context.Context, target string) (io.WriteCloser, error) {
	target = path.Join(f.base, target)
	err := os.MkdirAll(path.Dir(target), os.ModePerm)
	if err != nil {
		return nil, err
	}

	return os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
}

func (f *osfs) Reader(_ context.Context, target string) (io.Reader, error) {
	target = path.Join(f.base, target)
	err := os.MkdirAll(path.Dir(target), os.ModePerm)
	if err != nil {
		return nil, err
	}

	return os.OpenFile(target, os.O_CREATE|os.O_RDONLY, 0644)
}

type s3fs struct {
	bucket s3io.Bucket
}

func (f *s3fs) Writer(ctx context.Context, target string) (io.WriteCloser, error) {
	return f.bucket.Put(ctx, target), nil
}

func (f *s3fs) Reader(ctx context.Context, target string) (io.Reader, error) {
	return f.bucket.Get(ctx, target), nil
}

type multiWriteCloser []io.WriteCloser

func (t multiWriteCloser) Close() error {
	var err error
	for _, c := range t {
		err = errors.Join(c.Close())
	}

	return err
}

func (t multiWriteCloser) Write(p []byte) (n int, err error) {
	for _, w := range t {
		n, err = w.Write(p)
		if err != nil {
			return
		}
		if n != len(p) {
			err = io.ErrShortWrite
			return
		}
	}
	return len(p), nil
}
