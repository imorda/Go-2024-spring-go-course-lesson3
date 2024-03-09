package main

import "os"

type BetterStream os.File // This one has unknown size and doesn't support random access

func (stream BetterStream) Write(p []byte) (n int, err error) {
	o := os.File(stream)
	return o.Write(p)
}

func (stream BetterStream) Read(p []byte) (n int, err error) {
	o := os.File(stream)
	return o.Read(p)
}

func (stream BetterStream) Seek(offset int64, whence int) (int64, error) {
	if offset < 0 {
		return 0, invalidOperation
	}
	if whence != 1 { // Only relative seek forward is supported on a stream
		return 0, invalidOperation
	}
	pos := int64(0)
	for pos < offset {
		buff := make([]byte, offset-pos)
		n, err := stream.Read(buff)
		if err != nil {
			return pos, err
		}
		pos += int64(n)
	}
	return pos, nil
}

func (stream BetterStream) Close() error {
	o := os.File(stream)
	return o.Close()
}

func (stream BetterStream) Size() (int64, error) {
	return -1, invalidOperation
}
