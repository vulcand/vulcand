Vulcand
=======

* HTTP proxy that uses Etcd as a configuration backend.
* Changes to configuration take effect immediately without restarting the service.



![Vulcan diagram](http://coreos.com/assets/images/media/vulcan-1-upstream.png "Vulcan diagram")

Status
------
Hardening: Testing, benchmarking. Not usable for production yet.

Discussions
-----------

Mailing list: https://groups.google.com/d/forum/vulcan-proxy

Installation
------------

Deps: go>=1.2, Etcd

Install: 

```bash
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

E.g. location loc1 will serve the request `curl http://example.com/search` because it matches the path `/search` and host `example.com`:

![Vulcan diagram](http://coreos.com/assets/images/media/vulcan-diagram.png "Vulcan diagram")


Upstream
---------

Upstream is a collection of endpoints. Upstream can be assigned to multiple locations at the same time. This is convenient as sometimes one endpoint serves multiple purposes and locations.


Endpoint
---------

Endpoint is a final destination of the incoming request, each endpoint is defined by `<schema>://<host>:<port>`, e.g. `http://localhost:5000`

Middleware
----------

Vulcand supports pluggable middlewares. Middlewares can intercept or transform the request to any location. Examples of the supported middlewares are rate limits and connection limits.
You can add or remove middlewares using command line, API or directly via backends.

Etcd
====

Vulcan supports reading and updating configuration based on the changes in Etcd. Vulcans watch etcd prefix that is supplied when running an instance and configure themselves.
*Important* All examples bellow assume that Vulcand is configured to listen on `/vulcand` prefix, which is a default prefix.


Upstreams and endpoints
-----------------------

```bash
# Upserts upstream and adds an endpoint to it
etcdctl set /vulcand/upstreams/up1/endpoints/e1 http://localhost:5000
```

Hosts and locations
-------------------

```bash
# Upsert a host "localhost" and add a location to it that matches path "/home" and uses endpoints from upstream "up1"
etcdctl set /vulcand/hosts/localhost/locations/loc1/path "/home"
etcdctl set /vulcand/hosts/localhost/locations/loc1/upstream up1
```

The best part is that you can update upstream using the same command. Let's add another upstream and switch traffic to it:

```bash
# create a new upstream with endpoint http://localhost:5003
etcdctl set /vulcand/upstreams/up2/endpoints/e3 http://localhost:5003

# redirect the traffic of the location "loc1" to the endpoints of the upstream "up2"
etcdctl set /vulcand/hosts/localhost/locations/loc1/upstream up2
```

Note that you can add and remove endpoints to the existing upstream, and vulcan will start redirecting the traffic to them automatically:

```bash
# Add a new endpoint to the existing upstream
etcdctl set /vulcand/upstreams/up1/endpoints/e2 http://localhost:5001
```

Rate and connection limits
--------------------------

Vulcan supports setting rate and connection limits.

```bash
# Update or set rate limit the request to location "loc1" to 1 request per second per client ip with bursts up to 3 requests per second.
# Note the priority here - middlewares with lower priorities will be executed earlier.
etcdctl set /vulcand/hosts/localhost/locations/loc1/middlewares/ratelimit/rl1 '{"Type": "ratelimit", "Middleware":{"Requests":1, "PeriodSeconds":1, "Burst":3, "Variable": "client.ip"}}'
```

```bash
# Update or set the connection limit to 3 simultaneous connections per client ip at a time
# Note the priority here - middlewares with lower priorities will be executed earlier.
etcdctl set /vulcand/hosts/localhost/locations/loc1/middlewares/connlimit/rl1 '{"Type": "connlimit", "Middleware":{"Requests":1, "PeriodSeconds":1, "Burst":3, "Variable": "client.ip"}}'
```


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
Essentially it's a tiny wrapper around the Etcd backend, that writes to the Etcd.
Multiple Vulcand instances listening to the same prefix would detect changes simultaneously and reload configuration.

Status
----

```GET /v1/status```

Check status of the Vulcan instance.

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

Retrieve the existing hosts.

Example response:

```json
{
  "Hosts": [
    {
      "Name": "localhost",
      "Locations": [
        {
          "Hostname": "localhost",
          "Path": "/home",
          "Id": "loc1",
          "Upstream": {
            "Id": "up1",
            "Endpoints": [
              {
                "Id": "e1",
                "Url": "http://localhost:5000",
                "UpstreamId": "up1",
                "Stats": {
                  "Successes": 0,
                  "Failures": 0,
                  "FailRate": 0,
                  "PeriodSeconds": 10
                }
              }
            ]
          },
          "Middlewares": [
            {
              "Id": "rl1",
              "Priority": 0,
              "Type": "ratelimit",
              "Middleware": {
                "PeriodSeconds": 1,
                "Burst": 3,
                "Variable": "client.ip",
                "Requests": 1
              }
            }
          ]
        }
      ]
    }
  ]
}
```


```POST 'application/json' /v1/hosts```

Add a host to the proxy.

```json
{
  "Name": "localhost"
}
```

| Parameter     | Description   |
| ------------- |---------------|
| name          | Hostname      |


```DELETE /v1/hosts/<name>```

Delete a host.


Upstream
--------

```GET /v1/upstreams```

Retrieve the existing upstreams. Example response:

```json
{
  "Upstreams": [
    {
      "Id": "up1",
      "Endpoints": [
        {
          "Id": "e1",
          "Url": "http://localhost:5000",
          "UpstreamId": "up1",
          "Stats": null
        },
        {
          "Id": "e2",
          "Url": "http://localhost:5001",
          "UpstreamId": "up1",
          "Stats": null
        }
      ]
    }
  ]
}
```

```POST 'application/json' /v1/upstreams```

```json
{"Id": "up1"}
```

Add upstream to the proxy

| Parameter     | Description   |
| ------------- |---------------|
| id            | Optional upstream id, will be generated if omitted.      |

```DELETE /v1/upstreams/<id>```

Delete an upstream.


```GET /v1/upstreams/drain?timeout=3```

Wait till there are no more connections to any endpoints to the upstream.

| Parameter     | Description   |
| ------------- |---------------|
| timeout       | Timeout in form `1s` for the amount of seconds to wait before time out.      |

Example response:

```json
{
  "Connections": 0
}
```


Endpoint
--------

```GET /v1/upstreams/<id>/endpoints```

Retrieve the endpoints of the upstream. Example response:

```json
{
  "Endpoints": [
    {
      "Id": "e1",
      "Url": "http://localhost:5000",
      "UpstreamId": "up1"
    }
  ]
}
```

```GET /v1/upstreams/<id>/endpoints/<endpoint-id>```

Retrieve the particular endpoint with id `endpoint-id`


```POST /v1/upstreams/<id>/endpoints```

```json
{
  "Id": "e4",
  "Url": "http://localhost:5004",
  "UpstreamId": "up1"
}
```

Add endpoint to the upstream

| Parameter     | Description   |
| ------------- |---------------|
| id          | Optional endppint id, will be generated if omitted.|
| url         | Required valid endpoint url |


Example response:

```json
{
  "Id": "e4",
  "Url": "http://localhost:5004",
  "UpstreamId": "up1",
  "Stats": null
}
```


```DELETE /v1/upstreams/<id>/endpoints/<endpoint-id>```

Delete an endpoint.


Location
--------

```GET /v1/hosts/<hostname>/locations```

Retrieve the locations of the host.

Example response:

```json
{
  "Locations": [
    {
      "Hostname": "localhost",
      "Path": "/home",
      "Id": "loc1",
      "Upstream": {
        "Id": "up1",
        "Endpoints": [
          {
            "Id": "e1",
            "Url": "http://localhost:5000",
            "UpstreamId": "up1",
            "Stats": null
          }
        ]
      },
      "Middlewares": []
    }
  ]
}
```

```GET /v1/hosts/<hostname>/locations/<location-id>```

Retrieve the particular location in the host `hostname` with id `location-id`

```json
{
  "Hostname": "localhost",
  "Path": "/home",
  "Id": "loc1",
  "Upstream": {
    "Id": "up1",
    "Endpoints": [
      {
        "Id": "e1",
        "Url": "http://localhost:5000",
        "UpstreamId": "up1",
        "Stats": null
      },
      {
        "Id": "e2",
        "Url": "http://localhost:5001",
        "UpstreamId": "up1",
        "Stats": null
      },
      {
        "Id": "e3",
        "Url": "http://localhost:5003",
        "UpstreamId": "up1",
        "Stats": null
      },
      {
        "Id": "e4",
        "Url": "http://localhost:5004",
        "UpstreamId": "up1",
        "Stats": null
      }
    ]
  },
  "Middlewares": [
    {
      "Id": "rl1",
      "Priority": 0,
      "Type": "ratelimit",
      "Middleware": {
        "PeriodSeconds": 1,
        "Burst": 3,
        "Variable": "client.ip",
        "Requests": 1
      }
    },
    {
      "Id": "cl1",
      "Priority": 0,
      "Type": "connlimit",
      "Middleware": {
        "Connections": 3,
        "Variable": "client.ip"
      }
    }
  ]
}
```


```POST 'application/json' /v1/hosts/<hostname>/locations```

Add a location to the host

```json
{
  "Id": "loc2",
  "Hostname": "localhost",
  "Path": "/home",
  "Upstream": {
    "Id": "up1"
  }
}
```

| Parameter     | Description   |
| ------------- |---------------|
| Id            | Optional location id, will be generated if omitted.|
| Path          | Required regular expression for path matchng |
| Upstream.Id   | Required id of the existing upstream |
| Hostname      | Required hostname|


```DELETE /v1/hosts/<hostname>/locations/<location-id>```

Delete a location


```PUT /v1/hosts/<hostname>/locations/<location-id>```

Update location's upstream. Gracefully Redirects all the traffic to the endpoints of the new upstream.

| Parameter     | Description   |
| ------------- |---------------|
| upstream     | Required id of the existing upstream |


Rate limit
----------

```GET /v1/hosts/<hostname>/locations/<location-id>/middlewares/ratelimit/<rate-id>```

Retrieve the particular rate of location in the host `hostname` with id `location-id` and rate id `rate-id`

```json
{
  "Id": "rl1",
  "Priority": 0,
  "Type": "ratelimit",
  "Middleware": {
    "PeriodSeconds": 1,
    "Burst": 3,
    "Variable": "client.ip",
    "Requests": 1
  }
}
```


```POST 'application/json' /v1/hosts/<hostname>/locations/limits/rates```

Add a rate limit to the location, will take effect immediately.

```json
{
  "Id": "rl1",
  "Priority": 0,
  "Type": "ratelimit",
  "Middleware": {
    "PeriodSeconds": 1,
    "Burst": 3,
    "Variable": "client.ip",
    "Requests": 1
  }
}
```

| Parameter     | Description   |
| ------------- |---------------|
| Id            | Optional rate id, will be generated if omitted.|
| Requests      | Required amount of allowed requests|
| PeriodSeconds       | Required period in seconds for counting the requests |
| Burst         | Required allowed burst of the requests (additional requests exceeding the rate) |
| Variable      | Variable for rate limiting e.g. `client.ip` or `request.header.My-Header` |


```DELETE /v1/hosts/<hostname>/locations/<location-id>/limits/rates/<rate-id>```

Delete a rate limit from the location.


```PUT /v1/hosts/<hostname>/locations/<location-id>/limits/rates/<rate-id>```

Update location's rate limit. Takes effect immdediatelly.

```json
{
  "Id": "rl1",
  "Priority": 0,
  "Type": "ratelimit",
  "Middleware": {
    "PeriodSeconds": 1,
    "Burst": 3,
    "Variable": "client.ip",
    "Requests": 1
  }
}
```

Connection limit
-----------------

```GET /v1/hosts/<hostname>/locations/<location-id>/middlewares/connlimit/<conn-id>```

Retrieve the particular connection limit of location in the host `hostname` with id `location-id` and connection limit id `conn-id`

Example response:

```json
{
  "Id": "cl1",
  "Priority": 0,
  "Type": "connlimit",
  "Middleware": {
    "Connections": 3,
    "Variable": "client.ip"
  }
}
```

```POST 'application/json' /v1/hosts/<hostname>/locations/limits/connections```

Add a connection limit to the location, will take effect immediately.

```json
{
  "Id": "cl1",
  "Priority": 0,
  "Type": "connlimit",
  "Middleware": {
    "Connections": 3,
    "Variable": "client.ip"
  }
}
```

| Parameter     | Description   |
| ------------- |---------------|
| Id          | Optional limit id, will be generated if omitted.|
| Connections     | Required maximum amount of allowed simultaneous connections|
| Variable     | Variable for limiting e.g. `client.ip` or `request.header.My-Header` |


```DELETE /v1/hosts/<hostname>/locations/<location-id>/middlewares/connlimit/<conn-id>```

Delete a connection limit from the location.


```PUT /v1/hosts/<hostname>/locations/<location-id>/limits/connections/<conn-id>```

Update location's connection limit. Takes effect immdediatelly.

```json
{
  "Id": "cl1",
  "Priority": 0,
  "Type": "connlimit",
  "Middleware": {
    "Connections": 3,
    "Variable": "client.ip"
  }
}
```

Docker
======

Here's how you build vulcan in Docker:

```bash
docker build -t mailgun/vulcan .
```

Starting the daemon:

```bash
docker run -p 8182:8182 -p 8181:8181 mailgun/vulcan /opt/vulcan/vulcand -apiInterface="0.0.0.0" -interface="0.0.0.0" --etcd=http://10.0.3.1:4001
```

Don't forget to map the ports and bind to the proper interfaces, otherwise vulcan won't be reachable from outside the container.

Using the vulcanctl from container:

```bash
docker run mailgun/vulcan /opt/vulcan/vulcanctl status  --vulcan 'http://10.0.3.1:8182'
```

Make sure you've specified `--vulcan` flag to tell vulcanctl where the running vulcand is. I've used lxc bridge interface in the example above.

*Note* The dockerfile build in the example above is not reproducible (yet), and the vulcand API is a subject to change.
