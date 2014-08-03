package testutils

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"time"
)

type RndString struct {
	src rand.Source
}

func NewRndString() *RndString {
	return &RndString{rand.NewSource(time.Now().UTC().UnixNano())}
}

func (r *RndString) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = byte(r.src.Int63()%26 + 97)
	}
	return len(p), nil
}

func (r *RndString) MakeString(n int) string {
	buffer := &bytes.Buffer{}
	io.CopyN(buffer, r, int64(n))
	return buffer.String()
}

func (s *RndString) MakePath(varlen, minlen int) string {
	return fmt.Sprintf("/%s", s.MakeString(rand.Intn(varlen)+minlen))
}
