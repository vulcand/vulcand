package etcdbackend

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/go-etcd/etcd"
	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/secret"
)

// lazyClient caches the keys that have been read and uses recursive queries
// what helps to reduce the amount of read requests to the Etcd backend.
// It is recommended to use lazyClient only in the context of the single call
// to the public etcdbackend interface to avoid dealing with cache invalidation issues.
type lazyClient struct {
	nodes map[string]*etcd.Node
	b     *secret.Box
	c     *etcd.Client
}

func newLazyClient(c *etcd.Client, b *secret.Box) (*lazyClient, error) {
	client := &lazyClient{
		nodes: make(map[string]*etcd.Node),
		b:     b,
		c:     c,
	}
	return client, nil
}

// getNode will retreive the value from the backend and cache it for subsequent reads.
func (c *lazyClient) getNode(key string) (*etcd.Node, error) {
	for k, n := range c.nodes {
		if strings.HasPrefix(key, k) {
			return c.findNode(key, n)
		}
	}
	response, err := c.c.Get(key, true, true)
	if err != nil {
		return nil, convertErr(err)
	}
	c.nodes[key] = response.Node
	return c.findNode(key, response.Node)
}

// findNode searches the root node for the child with matching key.
func (c *lazyClient) findNode(key string, node *etcd.Node) (*etcd.Node, error) {
	if node == nil {
		return nil, &backend.NotFoundError{Message: fmt.Sprintf("key '%s' not found", key)}
	}

	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}

	if key == node.Key {
		return node, nil
	}
	if !isDir(node) {
		return nil, &backend.NotFoundError{Message: fmt.Sprintf("key '%s' not found", key)}
	}

	for _, child := range node.Nodes {
		n, err := c.findNode(key, child)
		if err == nil {
			return n, nil
		}
	}
	return nil, &backend.NotFoundError{Message: fmt.Sprintf("key '%s' not found", key)}
}

func (c *lazyClient) getVals(key string) ([]Pair, error) {
	var out []Pair
	n, err := c.getNode(key)
	if err != nil {
		if isNotFoundError(err) {
			return out, nil
		}
		return nil, err
	}
	if !isDir(n) {
		return out, nil
	}
	for _, srvNode := range n.Nodes {
		if !isDir(srvNode) {
			out = append(out, Pair{srvNode.Key, srvNode.Value})
		}
	}
	return out, nil
}

func (c *lazyClient) getVal(key string) (string, error) {
	n, err := c.getNode(key)
	if err != nil {
		return "", err
	}
	if isDir(n) {
		return "", &backend.NotFoundError{Message: fmt.Sprintf("expected value, got dir for key '%s'", key)}
	}
	return n.Value, nil
}

func (c *lazyClient) getSealedVal(key string) ([]byte, error) {
	bytes, err := c.getVal(key)
	if err != nil {
		return nil, err
	}
	if c.b == nil {
		return nil, fmt.Errorf("this backend does not support encryption bla: %s %s", bytes, err)
	}
	sv, err := secret.SealedValueFromJSON([]byte(bytes))
	if err != nil {
		return nil, err
	}
	return c.b.Open(sv)
}

func (c *lazyClient) getJSONVal(key string, in interface{}) error {
	val, err := c.getVal(key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), in)
}

func (c *lazyClient) checkKeyExists(key string) error {
	_, err := c.getNode(key)
	return err
}

func (c *lazyClient) getDirs(key string) ([]string, error) {
	var out []string
	n, err := c.getNode(key)
	if err != nil {
		if isNotFoundError(err) {
			return out, nil
		}
		return nil, err
	}

	if !isDir(n) {
		return out, nil
	}

	for _, srvNode := range n.Nodes {
		if isDir(srvNode) {
			out = append(out, srvNode.Key)
		}
	}
	return out, nil
}
