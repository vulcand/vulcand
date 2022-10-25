// package systest contains "Black box" tests that configure Vulcand using various methods and making sure
// Vulcand accepts the configuration and is capable of processing requests.
package systest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/testutils"
	"github.com/vulcand/vulcand/engine"
	v3 "github.com/vulcand/vulcand/engine/etcdng/v3"
	"github.com/vulcand/vulcand/secret"
	vulcanutils "github.com/vulcand/vulcand/testutils"
	etcd "go.etcd.io/etcd/client/v3"
	. "gopkg.in/check.v1"
)

func TestVulcandWithEtcd(t *testing.T) { TestingT(t) }

var (
	apiUrl     string
	serviceUrl string
	etcdNodes  []string
	etcdPrefix string
	sealKey    string
	box        *secret.Box
	client     *etcd.Client
)

func TestMain(m *testing.M) {
	var err error

	nodes := os.Getenv("VULCAND_TEST_ETCD_NODES")
	if nodes == "" {
		fmt.Println("This test requires running Etcd, please provide url via VULCAND_TEST_ETCD_NODES environment variable")
		return
	}

	etcdNodes = strings.Split(nodes, ",")
	client, err = etcd.New(etcd.Config{Endpoints: etcdNodes})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	etcdPrefix = os.Getenv("VULCAND_TEST_ETCD_PREFIX")
	if etcdPrefix == "" {
		fmt.Println("This test requires Etcd prefix, please provide url via VULCAND_TEST_ETCD_PREFIX environment variable")
		return
	}

	apiUrl = os.Getenv("VULCAND_TEST_API_URL")
	if apiUrl == "" {
		fmt.Println("This test requires running vulcand daemon, provide API url via VULCAND_TEST_API_URL environment variable")
		return
	}

	serviceUrl = os.Getenv("VULCAND_TEST_SERVICE_URL")
	if serviceUrl == "" {
		fmt.Println("This test requires running vulcand daemon, provide API url via VULCAND_TEST_SERVICE_URL environment variable")
		return
	}

	sealKey = os.Getenv("VULCAND_TEST_SEAL_KEY")
	if sealKey == "" {
		fmt.Println("This test requires running vulcand daemon, provide API url via VULCAND_TEST_SEAL_KEY environment variable")
		return
	}

	key, err := secret.KeyFromString(sealKey)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	box, err = secret.NewBox(key)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	ret := m.Run()
	exec.Command("killall", "vulcand").Output()
	os.Exit(ret)
}

func path(keys ...string) string {
	return strings.Join(append([]string{etcdPrefix}, keys...), "/")
}

func setUpTest(t *testing.T) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	// Delete all values under the given prefix
	_, err := client.Get(ctx, etcdPrefix)
	if err != nil && !v3.IsNotFound(err) {
		t.Errorf("Unexpected error: %v", err)
	}
	_, err = client.Delete(ctx, etcdPrefix, etcd.WithPrefix())

	// Restart vulcand
	exec.Command("killall", "vulcand").Run()

	args := []string{
		fmt.Sprintf("--etcdKey=%s", etcdPrefix),
		fmt.Sprintf("--sealKey=%s", sealKey),
		"--logSeverity=INFO",
	}
	for _, n := range etcdNodes {
		args = append(args, fmt.Sprintf("-etcd=%s", n))
	}
	cmd := exec.Command("vulcand", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Start())

	go func() {
		// Wait for process completion to avoid zombie process
		cmd.Wait()
	}()

	// Wait until vulcand is up and ready
	untilConnect(t, 10, time.Millisecond*300, "localhost:8182")

	return ctx, cancel
}

func TestFrontendCRUD(t *testing.T) {
	defer exec.Command("killall", "vulcand").Output()
	ctx, cancel := setUpTest(t)
	defer cancel()

	called := false
	server := testutils.NewHandler(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte("Hi, I'm fine, thanks!"))
	})
	defer server.Close()

	// Create a server
	b, srv, url := "bk1", "srv1", server.URL

	_, err := client.Put(ctx, path("backends", b, "backend"), `{"Type": "http"}`)
	require.NoError(t, err)

	_, err = client.Put(ctx, path("backends", b, "servers", srv),
		fmt.Sprintf(`{"URL": "%s"}`, url))
	require.NoError(t, err)

	// Add frontend
	fId := "fr1"
	_, err = client.Put(ctx, path("frontends", fId, "frontend"),
		`{"Type": "http", "BackendId": "bk1", "Route": "Path(\"/path\")"}`)
	require.NoError(t, err)

	time.Sleep(time.Second)
	resp, _, err := testutils.Get(fmt.Sprintf("%s%s", serviceUrl, "/path"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, called, true)
}

func TestFrontendUpdateLimits(t *testing.T) {
	defer exec.Command("killall", "vulcand").Output()
	ctx, cancel := setUpTest(t)
	defer cancel()

	server := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("Hello, I'm totally fine"))
	})
	defer server.Close()

	b, srv, url := "bk1", "srv1", server.URL
	_, err := client.Put(ctx, path("backends", b, "backend"),
		`{"Type": "http"}`)
	require.NoError(t, err)

	_, err = client.Put(ctx, path("backends", b, "servers", srv),
		fmt.Sprintf(`{"URL": "%s"}`, url))
	require.NoError(t, err)

	// Add frontend
	fId := "fr1"
	_, err = client.Put(ctx, path("frontends", fId, "frontend"),
		`{"Type": "http", "BackendId": "bk1", "Route": "Path(\"/path\")"}`)
	require.NoError(t, err)

	time.Sleep(time.Second)
	resp, _, err := testutils.Get(fmt.Sprintf("%s%s", serviceUrl, "/path"))
	require.NoError(t, err)

	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, resp.Header.Get("X-Forwarded-For"), "")

	_, err = client.Put(ctx, path("frontends", fId, "frontend"),
		`{"Type": "http", "BackendId": "bk1", "Route": "Path(\"/path\")", "Settings": {"Limits": {"MaxMemBodyBytes":2, "MaxBodyBytes":4}}}`)
	require.NoError(t, err)
	time.Sleep(time.Second)

	resp, _, err = testutils.Get(fmt.Sprintf("%s%s", serviceUrl, "/path"), testutils.Body("This is longer than allowed 4 bytes"))
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusRequestEntityTooLarge)
}

func TestFrontendUpdateBackend(t *testing.T) {
	defer exec.Command("killall", "vulcand").Output()
	ctx, cancel := setUpTest(t)
	defer cancel()

	server1 := testutils.NewHandler(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("1"))
	})
	defer server1.Close()

	server2 := testutils.NewHandler(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("2"))
	})
	defer server2.Close()

	// Create two different backends
	b1, srv1, url1 := "bk1", "srv1", server1.URL

	_, err := client.Put(ctx, path("backends", b1, "backend"),
		`{"Type": "http"}`)
	require.NoError(t, err)

	_, err = client.Put(ctx, path("backends", b1, "servers", srv1),
		fmt.Sprintf(`{"URL": "%s"}`, url1))
	require.NoError(t, err)

	b2, srv2, url2 := "bk2", "srv2", server2.URL
	_, err = client.Put(ctx, path("backends", b2, "backend"),
		`{"Type": "http"}`)
	require.NoError(t, err)

	_, err = client.Put(ctx, path("backends", b2, "servers", srv2),
		fmt.Sprintf(`{"URL": "%s"}`, url2))
	require.NoError(t, err)

	// Add frontend inititally pointing to the first backend
	fId := "fr1"
	_, err = client.Put(ctx, path("frontends", fId, "frontend"),
		`{"Type": "http", "BackendId": "bk1", "Route": "Path(\"/path\")"}`)
	require.NoError(t, err)

	time.Sleep(time.Second)
	url := fmt.Sprintf("%s%s", serviceUrl, "/path")
	resp, body, err := testutils.Get(url)
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, string(body), "1")

	// Update the backend
	_, err = client.Put(ctx, path("frontends", fId, "frontend"),
		`{"Type": "http", "BackendId": "bk2", "Route": "Path(\"/path\")"}`)
	require.NoError(t, err)

	time.Sleep(time.Second)
	resp, body, err = testutils.Get(url)
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, string(body), "2")
}

func TestHTTPListenerCRUD(t *testing.T) {
	defer exec.Command("killall", "vulcand").Output()
	ctx, cancel := setUpTest(t)
	defer cancel()

	called := false
	server := testutils.NewHandler(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte("Hi, I'm fine, thanks!"))
	})
	defer server.Close()

	b, srv, url := "bk1", "srv1", server.URL
	_, err := client.Put(ctx, path("backends", b, "backend"),
		`{"Type": "http"}`)
	require.NoError(t, err)

	_, err = client.Put(ctx, path("backends", b, "servers", srv),
		fmt.Sprintf(`{"URL": "%s"}`, url))
	require.NoError(t, err)

	// Add frontend
	fId := "fr1"
	_, err = client.Put(ctx, path("frontends", fId, "frontend"),
		`{"Type": "http", "BackendId": "bk1", "Route": "Path(\"/path\")"}`)
	require.NoError(t, err)

	time.Sleep(time.Second)
	resp, _, err := testutils.Get(fmt.Sprintf("%s%s", serviceUrl, "/path"))
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	// Add HTTP listener
	l1 := "l1"
	listener, err := engine.NewListener(l1, "http", "tcp", "localhost:31000", "", "", nil)
	require.NoError(t, err)
	bytes, err := json.Marshal(listener)
	require.NoError(t, err)
	client.Put(ctx, path("listeners", l1), string(bytes))

	time.Sleep(time.Second)
	_, _, err = testutils.Get(fmt.Sprintf("%s%s", "http://localhost:31000", "/path"))
	require.NoError(t, err)
	assert.Equal(t, called, true)

	_, err = client.Delete(ctx, path("listeners", l1), etcd.WithPrefix())
	require.NoError(t, err)

	time.Sleep(time.Second)

	_, _, err = testutils.Get(fmt.Sprintf("%s%s", "http://localhost:31000", "/path"))
	assert.Error(t, err)
}

func TestHTTPSListenerCRUD(t *testing.T) {
	defer exec.Command("killall", "vulcand").Output()
	ctx, cancel := setUpTest(t)
	defer cancel()

	called := false
	server := testutils.NewHandler(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte("Hi, I'm fine, thanks!"))
	})
	defer server.Close()

	b, srv, url := "bk1", "srv1", server.URL
	_, err := client.Put(ctx, path("backends", b, "backend"),
		`{"Type": "http"}`)
	require.NoError(t, err)

	_, err = client.Put(ctx, path("backends", b, "servers", srv),
		fmt.Sprintf(`{"URL": "%s"}`, url))
	require.NoError(t, err)

	// Add frontend
	fId := "fr1"
	_, err = client.Put(ctx, path("frontends", fId, "frontend"),
		`{"Type": "http", "BackendId": "bk1", "Route": "Path(\"/path\")"}`)
	require.NoError(t, err)

	keyPair := vulcanutils.NewTestKeyPair()

	bytes, err := secret.SealKeyPairToJSON(box, keyPair)
	require.NoError(t, err)
	sealed := base64.StdEncoding.EncodeToString(bytes)
	host := "localhost"

	_, err = client.Put(ctx, path("hosts", host, "host"),
		fmt.Sprintf(`{"Name": "localhost", "Settings": {"KeyPair": "%v"}}`, sealed))
	require.NoError(t, err)

	// Add HTTPS listener
	l2 := "ls2"
	listener, err := engine.NewListener(l2, "https", "tcp", "localhost:32000", "", "", nil)
	require.NoError(t, err)
	bytes, err = json.Marshal(listener)
	require.NoError(t, err)
	client.Put(ctx, path("listeners", l2), string(bytes))

	time.Sleep(time.Second)
	_, _, err = testutils.Get(fmt.Sprintf("%s%s", "https://localhost:32000", "/path"))
	require.NoError(t, err)
	assert.Equal(t, called, true)

	_, err = client.Delete(ctx, path("listeners", l2), etcd.WithPrefix())
	require.NoError(t, err)

	time.Sleep(time.Second)

	_, _, err = testutils.Get(fmt.Sprintf("%s%s", "https://localhost:32000", "/path"))
	assert.Error(t, err)
}

func TestExpiringServer(t *testing.T) {
	defer exec.Command("killall", "vulcand").Output()
	ctx, cancel := setUpTest(t)
	defer cancel()

	server := testutils.NewResponder("e1")
	defer server.Close()

	server2 := testutils.NewResponder("e2")
	defer server2.Close()

	// Create backend and servers
	b, url, url2 := "bk1", server.URL, server2.URL
	srv, srv2 := "s1", "s2"

	_, err := client.Put(ctx, path("backends", b, "backend"),
		`{"Type": "http"}`)
	require.NoError(t, err)

	// This one will stay
	_, err = client.Put(ctx, path("backends", b, "servers", srv),
		fmt.Sprintf(`{"URL": "%s"}`, url))
	require.NoError(t, err)

	// This one will expire
	lgr, err := client.Grant(ctx, int64(time.Second.Seconds()))
	require.NoError(t, err)

	_, err = client.Put(ctx, path("backends", b, "servers", srv2),
		fmt.Sprintf(`{"URL": "%s"}`, url2), etcd.WithLease(lgr.ID))
	require.NoError(t, err)

	// Add frontend
	fId := "fr1"
	_, err = client.Put(ctx, path("frontends", fId, "frontend"),
		`{"Type": "http", "BackendId": "bk1", "Route": "Path(\"/path\")"}`)
	require.NoError(t, err)

	time.Sleep(time.Second)
	resps1 := make(map[string]bool)
	for i := 0; i < 3; i += 1 {
		resp, body, err := testutils.Get(fmt.Sprintf("%s%s", serviceUrl, "/path"))
		require.NoError(t, err)
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		resps1[string(body)] = true
	}
	assert.Equal(t, resps1, map[string]bool{"e1": true, "e2": true})

	// Now the second endpoint should expire
	time.Sleep(time.Second * 4)

	resps2 := make(map[string]bool)
	for i := 0; i < 3; i += 1 {
		resp, body, err := testutils.Get(fmt.Sprintf("%s%s", serviceUrl, "/path"))
		require.NoError(t, err)
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		resps2[string(body)] = true
	}
	assert.Equal(t, map[string]bool{"e1": true}, resps2)
}

func TestBackendUpdateSettings(t *testing.T) {
	defer exec.Command("killall", "vulcand").Output()
	ctx, cancel := setUpTest(t)
	defer cancel()

	server := testutils.NewHandler(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte("tc: update upstream options"))
	})
	defer server.Close()

	b, srv, url := "bk1", "srv1", server.URL
	_, err := client.Put(ctx, path("backends", b, "backend"),
		`{"Type": "http", "Settings": {"Timeouts": {"Read":"10ms"}}}`)
	require.NoError(t, err)

	_, err = client.Put(ctx, path("backends", b, "servers", srv),
		fmt.Sprintf(`{"URL": "%s"}`, url))
	require.NoError(t, err)

	// Add frontend
	fId := "fr1"
	_, err = client.Put(ctx, path("frontends", fId, "frontend"),
		`{"Type": "http", "BackendId": "bk1", "Route": "Path(\"/path\")"}`)
	require.NoError(t, err)

	// Wait for the changes to take effect
	time.Sleep(time.Second)

	// Make sure request times out
	resp, _, err := testutils.Get(fmt.Sprintf("%s%s", serviceUrl, "/path"))
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusGatewayTimeout)

	// Update backend timeout
	_, err = client.Put(ctx, path("backends", b, "backend"),
		`{"Type": "http", "Settings": {"Timeouts": {"Read":"100ms"}}}`)
	require.NoError(t, err)

	// Wait for the changes to take effect
	time.Sleep(time.Second)

	resp, body, err := testutils.Get(fmt.Sprintf("%s%s", serviceUrl, "/path"))
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, string(body), "tc: update upstream options")
}

func untilConnect(t *testing.T, attempts int, waitTime time.Duration, addr string) {
	t.Helper()

	for i := 0; i < attempts; i++ {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			time.Sleep(waitTime)
			continue
		}
		conn.Close()
		return
	}
	t.Errorf("never connected to TCP server at '%s' after %d attempts", addr, attempts)
	t.FailNow()
}
