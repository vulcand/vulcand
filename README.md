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
$ vulcanctl host add localhost2
$ vulcanctl host rm localhost2
```

Upstream
--------

Upstream command adds or removes upstreams

```bash
$ vulcanctl upstream add u1
$ vulcanctl upstream rm u1
```

Endpoint
--------

Endpoint command adds or removed endpoints to the upstream.

```bash
$ vulcanctl endpoint add u1 e2 http://localhost:5002 # adds endpoint with id 'e2' and url 'http://localhost:5002' to upstream with id 'u1'
$ vulcanctl endpoint rm u1 e1 # removed endpoint with id 'e1' from upstream 'u1'
```

Location
--------

Location adds or removes location to the host

```bash
$ vulcanctl location add localhost loc1 /hello u1 # add location with id 'id1' to host 'localhost', use path '/hello' and upstream 'u1'
$ vulcanctl location rm localhost loc1 # remove location with id 'loc1' from host 'localhost'
```
