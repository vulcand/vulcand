package exproute

import (
	"bytes"
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/testutils"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	"net/http"
	"strings"
	"testing"
)

func TestTrie(t *testing.T) { TestingT(t) }

type TrieSuite struct {
}

var _ = Suite(&TrieSuite{})

func (s *TrieSuite) TestParseTrieSuccess(c *C) {
	t, l := makeTrie(c, "/", makeLoc("loc1"))
	c.Assert(t.match(makeReq("http://google.com")), Equals, l.location)
}

func (s *TrieSuite) TestParseTrieFailures(c *C) {
	paths := []string{
		"",                       // empty path
		"/<uint8:hi>",            // unsupported matcher
		"/<string:hi:omg:hello>", // unsupported matcher parameters
	}
	for _, path := range paths {
		l := &constMatcher{
			location: makeLoc("loc1"),
		}
		t, err := parseTrie(path, l)
		c.Assert(err, NotNil)
		c.Assert(t, IsNil)
	}
}

func (s *TrieSuite) testPathToTrie(c *C, path, trie string) {
	t, _ := makeTrie(c, path, makeLoc("loc1"))
	c.Assert(printTrie(t), Equals, trie)
}

func (s *TrieSuite) TestPrintTries(c *C) {
	// Simple path
	s.testPathToTrie(c, "/a", `
root
 node(/)
  match(a)
`)

	// Path wit default string parameter
	s.testPathToTrie(c, "/<param1>", `
root
 node(/)
  match(<string:param1>)
`)

	// Path with trailing parameter
	s.testPathToTrie(c, "/m/<string:param1>", `
root
 node(/)
  node(m)
   node(/)
    match(<string:param1>)
`)

	// Path with  parameter in the middle
	s.testPathToTrie(c, "/m/<string:param1>/a", `
root
 node(/)
  node(m)
   node(/)
    node(<string:param1>)
     node(/)
      match(a)
`)

	// Path with two parameters
	s.testPathToTrie(c, "/m/<string:param1>/<string:param2>", `
root
 node(/)
  node(m)
   node(/)
    node(<string:param1>)
     node(/)
      match(<string:param2>)
`)

}

func (s *TrieSuite) TestMergeTriesCommonPrefix(c *C) {
	t1, l1 := makeTrie(c, "/a", makeLoc("loc1"))
	t2, l2 := makeTrie(c, "/b", makeLoc("loc2"))

	t3, err := t1.merge(t2)
	c.Assert(err, IsNil)

	expected := `
root
 node(/)
  match(a)
  match(b)
`
	c.Assert(printTrie(t3.(*trie)), Equals, expected)

	c.Assert(t3.match(makeReq("http://google.com/a")), Equals, l1.location)
	c.Assert(t3.match(makeReq("http://google.com/b")), Equals, l2.location)
}

func (s *TrieSuite) TestMergeTriesSubtree(c *C) {
	t1, l1 := makeTrie(c, "/aa", makeLoc("loc1"))
	t2, l2 := makeTrie(c, "/a", makeLoc("loc2"))

	t3, err := t1.merge(t2)
	c.Assert(err, IsNil)

	expected := `
root
 node(/)
  match(a)
   match(a)
`
	c.Assert(printTrie(t3.(*trie)), Equals, expected)

	c.Assert(t3.match(makeReq("http://google.com/aa")), Equals, l1.location)
	c.Assert(t3.match(makeReq("http://google.com/a")), Equals, l2.location)
	c.Assert(t3.match(makeReq("http://google.com/b")), Equals, nil)
}

func (s *TrieSuite) TestMergeTriesWithCommonParameter(c *C) {
	t1, l1 := makeTrie(c, "/a/<string:name>/b", makeLoc("loc1"))
	t2, l2 := makeTrie(c, "/a/<string:name>/c", makeLoc("loc2"))

	t3, err := t1.merge(t2)
	c.Assert(err, IsNil)

	expected := `
root
 node(/)
  node(a)
   node(/)
    node(<string:name>)
     node(/)
      match(b)
      match(c)
`
	c.Assert(printTrie(t3.(*trie)), Equals, expected)

	c.Assert(t3.match(makeReq("http://google.com/a/bla/b")), Equals, l1.location)
	c.Assert(t3.match(makeReq("http://google.com/a/bla/c")), Equals, l2.location)
	c.Assert(t3.match(makeReq("http://google.com/a/")), IsNil)
}

func (s *TrieSuite) TestMergeTriesWithDivergedParameter(c *C) {
	t1, l1 := makeTrie(c, "/a/<string:name1>/b", makeLoc("loc1"))
	t2, l2 := makeTrie(c, "/a/<string:name2>/c", makeLoc("loc2"))

	t3, err := t1.merge(t2)
	c.Assert(err, IsNil)

	expected := `
root
 node(/)
  node(a)
   node(/)
    node(<string:name1>)
     node(/)
      match(b)
    node(<string:name2>)
     node(/)
      match(c)
`
	c.Assert(printTrie(t3.(*trie)), Equals, expected)

	c.Assert(t3.match(makeReq("http://google.com/a/bla/b")), Equals, l1.location)
	c.Assert(t3.match(makeReq("http://google.com/a/bla/c")), Equals, l2.location)
	c.Assert(t3.match(makeReq("http://google.com/a/")), IsNil)
}

func (s *TrieSuite) TestMergeTriesWithSamePath(c *C) {
	t1, l1 := makeTrie(c, "/a", makeLoc("loc1"))
	t2, _ := makeTrie(c, "/a", makeLoc("loc2"))

	t3, err := t1.merge(t2)
	c.Assert(err, IsNil)

	expected := `
root
 node(/)
  match(a)
`
	c.Assert(printTrie(t3.(*trie)), Equals, expected)
	// The first location will match as it will always go first
	c.Assert(t3.match(makeReq("http://google.com/a")), Equals, l1.location)
}

func (s *TrieSuite) TestMergeAndMatchCases(c *C) {
	testCases := []struct {
		trees    []string
		url      string
		expected string
	}{
		// Matching /
		{
			[]string{"/"},
			"http://google.com/",
			"/",
		},
		// Matching / when there's no trailing / in url
		{
			[]string{"/"},
			"http://google.com",
			"/",
		},
		// Choosing longest path
		{
			[]string{"/v2/domains/", "/v2/domains/domain1"},
			"http://google.com/v2/domains/domain1",
			"/v2/domains/domain1",
		},
		// Named parameters
		{
			[]string{"/v1/domains/<string:name>", "/v2/domains/<string:name>"},
			"http://google.com/v2/domains/domain1",
			"/v2/domains/<string:name>",
		},
		// Different combinations of named parameters
		{
			[]string{"/v1/domains/<domain>", "/v2/users/<user>/mailboxes/<mbx>"},
			"http://google.com/v2/users/u1/mailboxes/mbx1",
			"/v2/users/<user>/mailboxes/<mbx>",
		},
		// Something that looks like a pattern, but it's not
		{
			[]string{"/v1/<hello"},
			"http://google.com/v1/<hello",
			"/v1/<hello",
		},
	}
	for _, tc := range testCases {
		t, _ := makeTrie(c, tc.trees[0], makeLoc(tc.trees[0]))
		for i, pattern := range tc.trees {
			if i == 0 {
				continue
			}
			t2, _ := makeTrie(c, pattern, makeLoc(pattern))
			out, err := t.merge(t2)
			c.Assert(err, IsNil)
			t = out.(*trie)
		}
		out := t.match(makeReq(tc.url))
		c.Assert(out.(*location.ConstHttpLocation).Url, Equals, tc.expected)
	}
}

func (s *TrieSuite) BenchmarkMatching(c *C) {
	rndString := testutils.NewRndString()
	l := makeLoc("loc")

	t, _ := makeTrie(c, rndString.MakePath(20, 10), l)

	for i := 0; i < 10000; i++ {
		t2, _ := makeTrie(c, rndString.MakePath(20, 10), l)
		out, err := t.merge(t2)
		if err != nil {
			c.Assert(err, IsNil)
		}
		t = out.(*trie)
	}
	req := makeReq(fmt.Sprintf("http://google.com/%s", rndString.MakePath(20, 10)))
	for i := 0; i < c.N; i++ {
		t.match(req)
	}
}

func cutTrie(index int, expressions []string) []string {
	v := make([]string, 0, len(expressions)-1)
	v = append(v, expressions[:index]...)
	v = append(v, expressions[index+1:]...)
	return v
}

func mergeTries(c *C, expressions []string) *trie {
	t, _ := makeTrie(c, expressions[0], makeLoc(expressions[0]))
	for i, expression := range expressions {
		if i == 0 {
			continue
		}
		t2, _ := makeTrie(c, expression, makeLoc(expression))
		out, err := t.merge(t2)
		c.Assert(err, IsNil)
		t = out.(*trie)
	}
	return t
}

func makeTrie(c *C, path string, location location.Location) (*trie, *constMatcher) {
	l := &constMatcher{
		location: location,
	}
	t, err := parseTrie(path, l)
	c.Assert(err, IsNil)
	c.Assert(t, NotNil)
	return t, l
}

func makeReq(url string) request.Request {
	u := netutils.MustParseUrl(url)
	return &request.BaseRequest{
		HttpRequest: &http.Request{URL: u, RequestURI: url},
	}
}

func makeLoc(url string) location.Location {
	return &location.ConstHttpLocation{Url: url}
}

func printTrie(t *trie) string {
	return printTrieNode(t.root)
}

func printTrieNode(e *trieNode) string {
	out := &bytes.Buffer{}
	printTrieNodeInner(out, e, 0)
	return out.String()
}

func printTrieNodeInner(b *bytes.Buffer, e *trieNode, offset int) {
	if offset == 0 {
		fmt.Fprintf(b, "\n")
	}
	padding := strings.Repeat(" ", offset)
	fmt.Fprintf(b, "%s%s\n", padding, e.String())
	if len(e.children) != 0 {
		for _, c := range e.children {
			printTrieNodeInner(b, c, offset+1)
		}
	}
}
