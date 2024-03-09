package main

import "os"

type BetterFile os.File

func (file BetterFile) Write(p []byte) (n int, err error) {
	o := os.File(file)
	return o.Write(p)
}

func (file BetterFile) Read(p []byte) (n int, err error) {
	o := os.File(file)
	return o.Read(p)
}

func (file BetterFile) Seek(offset int64, whence int) (int64, error) {
	o := os.File(file)
	return o.Seek(offset, whence)
}

func (file BetterFile) Close() error {
	o := os.File(file)
	return o.Close()
}

func (file BetterFile) Size() (int64, error) {
	originFile := os.File(file)
	stat, err := originFile.Stat()
	return stat.Size(), err
}
