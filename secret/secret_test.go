package secret

import (
	"fmt"
	"testing"

	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestSecret(t *testing.T) { TestingT(t) }

type SecretSuite struct {
}

var _ = Suite(&SecretSuite{})

func (s *SecretSuite) TestEncryptDecryptCylce(c *C) {
	printableKey, err := NewPrintableKey()
	c.Assert(err, IsNil)

	key, err := DecodePrintableKey(printableKey)
	c.Assert(err, IsNil)

	b, err := NewBox(key)
	c.Assert(err, IsNil)

	message := []byte("hello, box!")
	encrypted, err := b.Encrypt(message)
	c.Assert(err, IsNil)
	fmt.Printf("%#v", encrypted)

	out, err := b.Decrypt(encrypted)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, message)
}
