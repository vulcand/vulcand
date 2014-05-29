package etcd

import "testing"

func TestAddChild(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.Delete("fooDir", true)
		c.Delete("nonexistentDir", true)
	}()

	c.CreateDir("fooDir", 5)

	_, err := c.AddChild("fooDir", "v0", 5)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.AddChild("fooDir", "v1", 5)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.Get("fooDir", true, false)
	// The child with v0 should proceed the child with v1 because it's added
	// earlier, so it should have a lower key.
	if !(len(resp.Node.Nodes) == 2 && (resp.Node.Nodes[0].Value == "v0" && resp.Node.Nodes[1].Value == "v1")) {
		t.Fatalf("AddChild 1 failed.  There should be two chlidren whose values are v0 and v1, respectively."+
			"  The response was: %#v", resp)
	}

	// Creating a child under a nonexistent directory should succeed.
	// The directory should be created.
	resp, err = c.AddChild("nonexistentDir", "foo", 5)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddChildDir(t *testing.T) {
	c := NewClient(nil)
	defer func() {
		c.Delete("fooDir", true)
		c.Delete("nonexistentDir", true)
	}()

	c.CreateDir("fooDir", 5)

	_, err := c.AddChildDir("fooDir", 5)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.AddChildDir("fooDir", 5)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.Get("fooDir", true, false)
	// The child with v0 should proceed the child with v1 because it's added
	// earlier, so it should have a lower key.
	if !(len(resp.Node.Nodes) == 2 && (len(resp.Node.Nodes[0].Nodes) == 0 && len(resp.Node.Nodes[1].Nodes) == 0)) {
		t.Fatalf("AddChildDir 1 failed.  There should be two chlidren whose values are v0 and v1, respectively."+
			"  The response was: %#v", resp)
	}

	child_resp, err := c.Get(resp.Node.Nodes[0].Key, false, false)
	// The child should actually be a directory
	if err != nil {
		t.Fatalf("AddChildDir 2 failed.  Getting one of the created children resulted in an error:", err)
	}
	if child_resp == nil {
		t.Fatalf("AddChildDir 2 failed.  Getting one of the created children resulted in a nil response, but no error.")
	}
	if child_resp.Node.Dir != true {
		t.Fatalf("AddChildDir 2 failed.  The created key should be a directory. The created node is: %#v", child_resp.Node)
	}

	// Creating a child under a nonexistent directory should succeed.
	// The directory should be created.
	resp, err = c.AddChildDir("nonexistentDir", 5)
	if err != nil {
		t.Fatal(err)
	}
}
