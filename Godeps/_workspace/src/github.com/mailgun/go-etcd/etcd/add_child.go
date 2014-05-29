package etcd

// Add a new directory with a random etcd-generated key under the given path.
func (c *Client) AddChildDir(key string, ttl uint64) (*Response, error) {
	ops := Options{
		"dir": true,
	}

	raw, err := c.post(key, "", ttl, ops)

	if err != nil {
		return nil, err
	}

	return raw.Unmarshal()
}

// Add a new file with a random etcd-generated key under the given path.
func (c *Client) AddChild(key string, value string, ttl uint64) (*Response, error) {
	raw, err := c.post(key, value, ttl, Options{})

	if err != nil {
		return nil, err
	}

	return raw.Unmarshal()
}
