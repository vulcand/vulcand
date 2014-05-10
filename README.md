Vulcand
=======

* HTTP proxy that uses Etcd as a configuration backend.
* Changes to configuration take effect immediately without restarting the service.

Status: Moving fast, breaking things. Will be usable soon, though.

Discussions
-----------

Mailing list: https://groups.google.com/d/forum/vulcan-proxy

Installation
------------

Deps: go>=1.2, Etcd

Install: 

```bash
make deps
make install
make run
```

Concepts
========

Host
----

Incoming requests are matched by their hostname first. Hostname is defined by incoming 'Host' header.
E.g. `curl http://example.com/alice` will be matched by the host `example.com` first.

Location
--------
Hosts contain one or several locations. Each location defines a path - simply a regular expression that will be matched against request's url.
Location contains link to an upstream and vulcand will use the endpoints from this upstream to serve the request.

E.g. location loc1 will serve the request `curl http://example.com/alice` because it matches the path `/alice`:

```
    └location(id=loc1, path=/alice)
      │
      ...
```


Upstream
---------

Upstream is a collection of endpoints. Upstream can be assigned to multiple locations at the same time. This is convenient as sometimes one endpoint serves multiple 
purposes and locations.


Endpoint
---------

Endpoint is a final destination of the incoming request, each endpoint is defined by `<schema>://<host>:<port>`, e.g. `http://localhost:5000`


Command line
============

Command line is the most convenient way to start working with vulcan, here are some examples. 

Status
------

Displays the configuration and stats about the daemon

```bash 
$ vulcanctl status

[hosts]
  │
  └host(name=localhost)
    │
    └location(id=loc1, path=/hello)
      │
      └upstream(id=u1)
        │
        └endpoint(id=e1, url=http://localhost:5001)
```

Host
----

Host command supports adding or removing host

```bash
# Add host with name 'example.com'
$ vulcanctl host add --name example.com

# Remove host with name 'example.com'
$ vulcanctl host rm --name example.com
```

Upstream
--------

Upstream command adds or removes upstreams

```bash
# Add upstream  with id 'u1'
$ vulcanctl upstream add --id u1

# Adds upstream with auto generated id
$ vulcanctl upstream add 

# Remove upstream with id 'u1'
$ vulcanctl upstream rm --id u1

# "Drain" - wait till there are no more active connections from the endpoints of the upstream 'u1'
# or timeout after 10 seconds if there are remaining connections
vulcanctl upstream drain -id u1 -timeout 10
```

Endpoint
--------

Endpoint command adds or removed endpoints to the upstream.

```bash
# add endpoint with id 'e2' and url 'http://localhost:5002' to upstream with id 'u1'
$ vulcanctl endpoint add --id e1 --up u1 --url http://localhost:5000 

# in case if id is omitted, etcd will auto generate it
$ vulcanctl endpoint add --up u1 --url http://localhost:5001 

# removed endpoint with id 'e1' from upstream 'u1'
$ vulcanctl endpoint rm --up u1 --id e1 
```

Location
--------

Location adds or removes location to the host

```bash
# add location with id 'id1' to host 'example.com', use path '/hello' and upstream 'u1'
$ vulcanctl location add --host example.com --id loc1 --path /hello --up u1 

# add location with auto generated id to host 'example.com', use path '/hello2' and upstream 'u1'
$ vulcanctl location add --host example.com --path /hello2 --up u1 

# remove location with id 'loc1' from host 'example.com'
$ vulcanctl location rm --host example.com --id loc1 

# update upstream of the location 'loc1' in host 'example.com' to be 'u2'
# this redirects the traffic gracefully from endpoints in the previous upstream
# to endpoints of the upstream 'u2', see drain for connection draining
$ vulcanctl location set_upstream --host example.com --id loc1 --up u2
```

Rate limit
----------

Rate add or removes rate limit restrictions on the location

```bash
# limit access per client ip to 10 requests per second in 
# location 'loc1' in host 'example.com'
$ vulcanctl ratelimit add --variable client.ip --host example.com --loc loc1 --requests 10

# limit access per custom http header value 'X-Account-Id' to 100 requests per second 
# to location 'loc1' in host 'example.com'
$ vulcanctl ratelimit add --variable request.header.X-Account-Id --host example.com --loc loc1 --requests 10

# remove rate limit restriction with id 'r1' from host 'example.com' location 'loc1'
$ vulcanctl ratelimit rm --id r1  --host example.com --loc 'loc1'
```

Connection limit
-----------------

Control simultaneous connections for a location.

```bash
# limit access per client ip to 10 simultaneous connections for
# location 'loc1' in host 'example.com'
$ vulcanctl connlimit add --id c1 -host example.com -loc loc1 -connections 10

# limit access per custom http header value 'X-Account-Id' to 100 simultaneous connections
# to location 'loc1' in host 'example.com'
$ vulcanctl connlimit add --variable request.header.X-Account-Id --host example.com --loc loc1 --connections 10

# remove connection limit restriction with id 'c1' from host 'example.com' location 'loc1'
$ vulcanctl connlimit rm --id c1  --host example.com --loc 'loc1'
```

HTTP API
========

Vulcan's HTTP API is the best way to configure one or several instances of Vulcan at the same time.  

Status
----

```GET /v1/status```

Check status of the vulcan process.

Returns:

```json
200 OK
{
   "Status": "ok"
}
```


Host
----

```GET /v1/hosts```

Retrieve the existing hosts


```POST /v1/hosts```

Add a host to the proxy

| Parameter     | Description   |
| ------------- |---------------|
| name          | Hostname      |


```DELETE /v1/hosts/<name>```

Delete a host.


Upstream
--------

```GET /v1/upstreams```

Retrieve the existing upstreams


```POST /v1/upstreams```

Add upstream to the proxy

| Parameter     | Description   |
| ------------- |---------------|
| id          | Optional upstream id, will be generated if omitted.      |

```DELETE /v1/upstreams/<id>```

Delete an upstream.


```GET /v1/upstreams/drain```

Wait till there are no more connections to any endpoints to the upstream.

| Parameter     | Description   |
| ------------- |---------------|
| timeout       | Timeout in form `1s` for the amount of seconds to wait before time out.      |


Endpoint
--------

```GET /v1/upstreams/<id>/endpoints```

Retrieve the endpoints of the upstream.


```GET /v1/upstreams/<id>/endpoints/<endpoint-id>```

Retrieve the particular endpoint with id `endpoint-id`


```POST /v1/upstreams/<id>/endpoints```

Add endpoint to the upstream

| Parameter     | Description   |
| ------------- |---------------|
| id          | Optional endppint id, will be generated if omitted.|
| url         | Required valid endpoint url |


```DELETE /v1/upstreams/<id>/endpoints/<endpoint-id>```

Delete an endpoint.


Location
--------

```GET /v1/hosts/<hostname>/locations```

Retrieve the locations of the host.


```GET /v1/hosts/<hostname>/locations/<location-id>```

Retrieve the particular location in the host `hostname` with id `location-id`


```POST /v1/hosts/<hostname>/locations```

Add a location to the host

| Parameter     | Description   |
| ------------- |---------------|
| id          | Optional location id, will be generated if omitted.|
| path         | Required regular expression for path matchng |
| upstream     | Required id of the existing upstream |


```DELETE /v1/hosts/<hostname>/locations/<location-id>```

Delete a location


```PUT /v1/hosts/<hostname>/locations/<location-id>```

Update location's upstream. Gracefully Redirects all the traffic to the endpoints of the new upstream.

| Parameter     | Description   |
| ------------- |---------------|
| upstream     | Required id of the existing upstream |


Rate limit
----------

```GET /v1/hosts/<hostname>/locations/<location-id>/limits/rates```

Retrieve the rate limits active for the location.


```GET /v1/hosts/<hostname>/locations/<location-id>/limits/rates/<rate-id>```

Retrieve the particular rate of location in the host `hostname` with id `location-id` and rate id `rate-id`


```POST /v1/hosts/<hostname>/locations/limits/rates```

Add a rate limit to the location, will take effect immediately.

| Parameter     | Description   |
| ------------- |---------------|
| id          | Optional rate id, will be generated if omitted.|
| requests     | Required amount of allowed requests|
| seconds     | Required period in seconds for counting the requests |
| burst     | Required allowed burst of the requests (additional requests exceeding the rate) |
| variable     | Variable for rate limiting e.g. `client.ip` or `request.header.My-Header` |


```DELETE /v1/hosts/<hostname>/locations/<location-id>/limits/rates/<rate-id>```

Delete a rate limit from the location.


```PUT /v1/hosts/<hostname>/locations/<location-id>/limits/rates/<rate-id>```

Update location's rate limit. Takes effect immdediatelly.

| Parameter     | Description   |
| ------------- |---------------|
| requests     | Required amount of allowed requests|
| seconds     | Required period in seconds for counting the requests |
| burst     | Required allowed burst of the requests (additional requests exceeding the rate) |
| variable     | Variable for rate limiting e.g. `client.ip` or `request.header.My-Header` |


Connection limit
-----------------

```GET /v1/hosts/<hostname>/locations/<location-id>/limits/connections```

Retrieve the connection limits active for the location.


```GET /v1/hosts/<hostname>/locations/<location-id>/limits/connections/<conn-id>```

Retrieve the particular connection limit of location in the host `hostname` with id `location-id` and connection limit id `conn-id`


```POST /v1/hosts/<hostname>/locations/limits/connections```

Add a connection limit to the location, will take effect immediately.

| Parameter     | Description   |
| ------------- |---------------|
| id          | Optional limit id, will be generated if omitted.|
| connections     | Required maximum amount of allowed simultaneous connections|
| variable     | Variable for limiting e.g. `client.ip` or `request.header.My-Header` |


```DELETE /v1/hosts/<hostname>/locations/<location-id>/limits/connections/<conn-id>```

Delete a connection limit from the location.


```PUT /v1/hosts/<hostname>/locations/<location-id>/limits/connections/<conn-id>```

Update location's connection limit. Takes effect immdediatelly.

| Parameter     | Description   |
| ------------- |---------------|
| connections     | Required maximum amount of allowed simultaneous connections|
| variable     | Variable for rate limiting e.g. `client.ip` or `request.header.My-Header` |


Docker
======

Warning: Docker (and vulcan builds are not reproducible yet!)

Here's how you build vulcan in Docker:

```bash
docker build -t mailgun/vulcan .
```

Starting the daemon:

```bash
docker run -p 8182:8182 -p 8181:8181 mailgun/vulcan /opt/vulcan/vulcand -apiInterface="0.0.0.0" -interface="0.0.0.0" --etcd=http://10.0.3.1:7002
```

Don't forget to map the ports and bind to the proper interfaces, otherwise vulcan won't be reachable from outside the container.

Using the vulcanctl from container:

```bash
docker run mailgun/vulcan /opt/vulcan/vulcanctl status  --vulcan 'http://10.0.3.1:8182'
```

Make sure you've specified `--vulcan` flag to tell vulcanctl where the running vulcand is. I've used lxc bridge interface in the example above.

*Note* The dockerfile build in the example above is not reproducible (yet), and the vulcand API is a subject to change.

