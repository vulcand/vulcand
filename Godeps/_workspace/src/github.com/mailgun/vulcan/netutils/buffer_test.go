package netutils

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	. "gopkg.in/check.v1"
	"io"
	"io/ioutil"
	"os"
)

type BufferSuite struct{}

var _ = Suite(&BufferSuite{})

func createReaderOfSize(size int64) (reader io.Reader, hash string) {
	f, err := os.Open("/dev/urandom")
	if err != nil {
		panic(err)
	}

	b := make([]byte, int(size))

	_, err = io.ReadFull(f, b)

	if err != nil {
		panic(err)
	}

	h := md5.New()
	h.Write(b)
	return bytes.NewReader(b), hex.EncodeToString(h.Sum(nil))
}

func hashOfReader(r io.Reader) string {
	h := md5.New()
	tr := io.TeeReader(r, h)
	_, _ = io.Copy(ioutil.Discard, tr)
	return hex.EncodeToString(h.Sum(nil))
}

func (s *BufferSuite) TestSmallBuffer(c *C) {
	r, hash := createReaderOfSize(1)
	bb, err := NewBodyBuffer(r)
	c.Assert(err, IsNil)
	c.Assert(hashOfReader(bb), Equals, hash)
	bb.Close()
}

func (s *BufferSuite) TestBigBuffer(c *C) {
	r, hash := createReaderOfSize(13631488)
	bb, err := NewBodyBuffer(r)
	c.Assert(err, IsNil)
	c.Assert(hashOfReader(bb), Equals, hash)
}

func (s *BufferSuite) TestSeek(c *C) {
	tlen := int64(1057576)
	r, hash := createReaderOfSize(tlen)
	bb, err := NewBodyBuffer(r)

	c.Assert(err, IsNil)
	c.Assert(hashOfReader(bb), Equals, hash)
	l, err := bb.TotalSize()
	c.Assert(err, IsNil)
	c.Assert(l, Equals, tlen)

	bb.Seek(0, 0)
	c.Assert(hashOfReader(bb), Equals, hash)
	l, err = bb.TotalSize()
	c.Assert(err, IsNil)
	c.Assert(l, Equals, tlen)
}

func (s *BufferSuite) TestSeekFirst(c *C) {
	tlen := int64(1057576)
	r, hash := createReaderOfSize(tlen)
	bb, err := NewBodyBuffer(r)

	l, err := bb.TotalSize()
	c.Assert(err, IsNil)
	c.Assert(l, Equals, tlen)

	c.Assert(err, IsNil)
	c.Assert(hashOfReader(bb), Equals, hash)

	bb.Seek(0, 0)

	c.Assert(hashOfReader(bb), Equals, hash)
	l, err = bb.TotalSize()
	c.Assert(err, IsNil)
	c.Assert(l, Equals, tlen)
}

func (s *BufferSuite) TestLimitDoesNotExceed(c *C) {
	requestSize := int64(1057576)
	r, hash := createReaderOfSize(requestSize)
	bb, err := NewBodyBufferWithOptions(r, BodyBufferOptions{MemBufferBytes: 1024, MaxSizeBytes: requestSize + 1})
	c.Assert(err, IsNil)
	c.Assert(hashOfReader(bb), Equals, hash)
	size, err := bb.TotalSize()
	c.Assert(err, IsNil)
	c.Assert(size, Equals, requestSize)
	bb.Close()
}

func (s *BufferSuite) TestLimitExceeds(c *C) {
	requestSize := int64(1057576)
	r, _ := createReaderOfSize(requestSize)
	bb, err := NewBodyBufferWithOptions(r, BodyBufferOptions{MemBufferBytes: 1024, MaxSizeBytes: requestSize - 1})
	c.Assert(err, FitsTypeOf, &MaxSizeReachedError{})
	c.Assert(bb, IsNil)
}

func (s *BufferSuite) TestWriteToBigBuffer(c *C) {
	l := int64(13631488)
	r, hash := createReaderOfSize(l)
	bb, err := NewBodyBuffer(r)
	c.Assert(err, IsNil)

	other := &bytes.Buffer{}
	wrote, err := bb.WriteTo(other)
	c.Assert(err, IsNil)
	c.Assert(wrote, Equals, l)
	c.Assert(hashOfReader(other), Equals, hash)
}

func (s *BufferSuite) TestWriteToSmallBuffer(c *C) {
	l := int64(1)
	r, hash := createReaderOfSize(l)
	bb, err := NewBodyBuffer(r)
	c.Assert(err, IsNil)

	other := &bytes.Buffer{}
	wrote, err := bb.WriteTo(other)
	c.Assert(err, IsNil)
	c.Assert(wrote, Equals, l)
	c.Assert(hashOfReader(other), Equals, hash)
}
