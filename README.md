Vulcand
=======

Vulcand is a programmatic extendable proxy for microservices and API management.
It is inspired by [Hystrix](https://github.com/Netflix/Hystrix) and powers Mailgun microservices infrastructure.

Focus and priorities
--------------------
Vulcand is focused on microservices and API use-cases.

Features
--------

* Uses Etcd as a configuration backend.
* API and command line tool.
* Pluggable middlewares.
* Support for canary deploys, realtime metrics and resiliency.

![Vulcan diagram](http://coreos.com/assets/images/media/vulcan-1-upstream.png "Vulcan diagram")

Project info
------------

| documentation | http://vulcand.github.io/                                   |
| :------------- |:-----------------------------------------------------------------|
| status        | Used in production@Mailgun on moderate workloads.  Under active development.              |
| discussions   | https://groups.google.com/d/forum/vulcan-proxy                  |
| roadmap       | [roadmap.md](ROADMAP.md)                  |
| build status  | [![Build Status](https://travis-ci.org/vulcand/vulcand.svg?branch=master)](https://travis-ci.org/vulcand/vulcand) |


Opentracing Support
------------
Vulcand has support for open tracing via the [Jaeger client
libraries](https://github.com/jaegertracing/jaeger-client-go). Users who wish
to use tracing support should use the `--enableJaegerTracing` flag and must
either run the Jaeger client listening on `localhost:6831/udp` or set the
environment variables `JAEGER_AGENT_HOST` and `JAEGER_AGENT_POST`. (See the
[Jaeger client libraries](https://github.com/jaegertracing/jaeger-client-go)
for all available configuration environment variables.

When enabled vulcand will create 2 spans, one span called `vulcand` which
covers the entire downstream request. The other span called `middleware` which
only spans the processing of the middleware before the request is routed
downstream.

Aliased Expressions
------------
When running vulcand in a kubernetes DaemonSet vulcand needs to know requests
from the local node can match `Host("localhost")` rules. This `--aliases` flag
allows an author of a vulcand DaemonSet to tell vulcand the name of the node it's
currently running on, such that vulcand correctly routes requests for
`Host("localhost")`. The `--aliases` flag allows the user to pass in multiple
aliases separated by comma's.

Example
```
$ vulcand --aliases 'Host("localhost")=Host("192.168.1.1")'
```
