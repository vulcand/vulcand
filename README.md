Vulcand
=======

* HTTP proxy that uses Etcd as a configuration backend.
* Changes to configuration take effect immediately without restarting the service.

Status
------

* Moving fast, breaking things.

Deps
----

* Etcd
* go>=1.2

Installation & Run
------------------

System deps:

* Linux, Etcd, go1.2

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
```

Endpoint
--------

Endpoint command adds or removed endpoints to the upstream.

```bash
$ vulcanctl endpoint add --id e1 --up u1 --url http://localhost:5000 # adds endpoint with id 'e2' and url 'http://localhost:5002' to upstream with id 'u1'
$ vulcanctl endpoint add --up u1 --url http://localhost:5001 # in case if id is omitted, etcd will auto generate it
$ vulcanctl endpoint rm --up u1 --id e1 # removed endpoint with id 'e1' from upstream 'u1'
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
```

Rate limit
----------

Rate add or removes rate limit restrictions on the location

```bash
# limit access per client ip to 10 requests per second in location 'loc1' in host 'example.com'
$ vulcanctl ratelimit add --variable client.ip --host example.com --loc loc1 --reqs 10 

# limit access per custom http header value 'X-Account-Id' to 100 requests per second to location 'loc1' in host 'example.com'
$ vulcanctl ratelimit add --variable request.header.X-Account-Id --host example.com --loc loc1 --reqs 10 

# remove rate limit restriction with id 'r1' from host 'example.com' location 'loc1'
$ vulcanctl ratelimit rm --id r1  --host example.com --loc 'loc1'
```

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

