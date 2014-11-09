package netutils

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

// MultiReader provides Read, Close, Seek and TotalSize methods. In addition to that it supports WriterTo interface
// to provide efficient writing schemes, as functions like io.Copy use WriterTo when it's available.
type MultiReader interface {
	io.Reader
	io.Seeker
	io.Closer
	io.WriterTo

	// TotalSize calculates and returns the total size of the reader and not the length remaining.
	TotalSize() (int64, error)
}

const (
	DefaultMemBufferBytes = 1048576
	DefaultMaxSizeBytes   = -1
	// Equivalent of bytes.MinRead used in ioutil.ReadAll
	DefaultBufferBytes = 512
)

// Constraints:
//  - Implements io.Reader
//  - Implements Seek(0, 0)
//	- Designed for Write once, Read many times.
type multiReaderSeek struct {
	length  int64
	readers []io.ReadSeeker
	mr      io.Reader
	cleanup CleanupFunc
}

type CleanupFunc func() error

func NewMultiReaderSeeker(length int64, cleanup CleanupFunc, readers ...io.ReadSeeker) *multiReaderSeek {
	converted := make([]io.Reader, len(readers))
	for i, r := range readers {
		// This conversion is safe as ReadSeeker includes Reader
		converted[i] = r.(io.Reader)
	}

	return &multiReaderSeek{
		length:  length,
		readers: readers,
		mr:      io.MultiReader(converted...),
		cleanup: cleanup,
	}
}

func (mr *multiReaderSeek) Close() (err error) {
	if mr.cleanup != nil {
		return mr.cleanup()
	}
	return nil
}

func (mr *multiReaderSeek) WriteTo(w io.Writer) (int64, error) {
	b := make([]byte, DefaultBufferBytes)
	var total int64
	for {
		n, err := mr.mr.Read(b)
		// Recommended way is to always handle non 0 reads despite the errors
		if n > 0 {
			nw, errw := w.Write(b[:n])
			total += int64(nw)
			// Write must return a non-nil error if it returns nw < n
			if nw != n || errw != nil {
				return total, errw
			}
		}
		if err != nil {
			if err == io.EOF {
				return total, nil
			}
			return total, err
		}
	}
}

func (mr *multiReaderSeek) Read(p []byte) (n int, err error) {
	return mr.mr.Read(p)
}

func (mr *multiReaderSeek) TotalSize() (int64, error) {
	return mr.length, nil
}

func (mr *multiReaderSeek) Seek(offset int64, whence int) (int64, error) {
	// TODO: implement other whence
	// TODO: implement real offsets

	if whence != 0 {
		return 0, fmt.Errorf("multiReaderSeek: unsupported whence")
	}

	if offset != 0 {
		return 0, fmt.Errorf("multiReaderSeek: unsupported offset")
	}

	for _, seeker := range mr.readers {
		seeker.Seek(0, 0)
	}

	ior := make([]io.Reader, len(mr.readers))
	for i, arg := range mr.readers {
		ior[i] = arg.(io.Reader)
	}
	mr.mr = io.MultiReader(ior...)

	return 0, nil
}

type BodyBufferOptions struct {
	// MemBufferBytes sets up the size of the memory buffer for this request.
	// If the data size exceeds the limit, the remaining request part will be saved on the file system.
	MemBufferBytes int64
	// Max size bytes, ignored if set to value <= 0, if request exceeds the specified limit, the reader will fail.
	MaxSizeBytes int64
}

func NewBodyBuffer(input io.Reader) (MultiReader, error) {
	return NewBodyBufferWithOptions(
		input, BodyBufferOptions{
			MemBufferBytes: DefaultMemBufferBytes,
			MaxSizeBytes:   DefaultMaxSizeBytes,
		})
}

func NewBodyBufferWithOptions(input io.Reader, o BodyBufferOptions) (MultiReader, error) {
	memReader := &io.LimitedReader{
		R: input,            // Read from this reader
		N: o.MemBufferBytes, // Maximum amount of data to read
	}
	readers := make([]io.ReadSeeker, 0, 2)

	buffer, err := ioutil.ReadAll(memReader)
	if err != nil {
		return nil, err
	}
	readers = append(readers, bytes.NewReader(buffer))

	var file *os.File
	// This means that we have exceeded all the memory capacity and we will start buffering the body to disk.
	totalBytes := int64(len(buffer))
	if memReader.N <= 0 {
		file, err = ioutil.TempFile("", "vulcan-bodies-")
		if err != nil {
			return nil, err
		}
		os.Remove(file.Name())

		readSrc := input
		if o.MaxSizeBytes > 0 {
			readSrc = &MaxReader{R: input, Max: o.MaxSizeBytes - o.MemBufferBytes}
		}

		writtenBytes, err := io.Copy(file, readSrc)
		if err != nil {
			return nil, err
		}
		totalBytes += writtenBytes
		file.Seek(0, 0)
		readers = append(readers, file)
	}

	var cleanupFn CleanupFunc
	if file != nil {
		cleanupFn = func() error {
			file.Close()
			return nil
		}
	}
	return NewMultiReaderSeeker(totalBytes, cleanupFn, readers...), nil
}

// MaxReader does not allow to read more than Max bytes and returns error if this limit has been exceeded.
type MaxReader struct {
	R   io.Reader // underlying reader
	N   int64     // bytes read
	Max int64     // max bytes to read
}

func (r *MaxReader) Read(p []byte) (int, error) {
	readBytes, err := r.R.Read(p)
	if err != nil && err != io.EOF {
		return readBytes, err
	}

	r.N += int64(readBytes)
	if r.N > r.Max {
		return readBytes, &MaxSizeReachedError{MaxSize: r.Max}
	}
	return readBytes, err
}

type MaxSizeReachedError struct {
	MaxSize int64
}

func (e *MaxSizeReachedError) Error() string {
	return fmt.Sprintf("Maximum size %d was reached", e)
}
