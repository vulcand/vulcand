# Roadmap

The goal of the roadmap is to provide some insight on what to expect from Vulcand in the short term
and long term.

## Short-term

### Releases

Support consistent semVer releases via Github

### Bugfixes

* Bugfixes/improvements in load balancing logic, circuit breakers, metrics

## Longer-term

### Routing

* Move to front-end and backend from hosts and locations
* Support pods/consistent hash-based routing
* Fan-In, Fan-Out support

### Reliability and performance

* TLS session caching
* Connection control for HTTP transports
* Reusing memory buffers with sync.Pool
* Profiling and benchmarking
* HTTP/2 support

### Reporting and UI

* Structured logging, ES connectors
* Dashboard with real-time metrics & CRUD
* Dependency analysis and visualization
* Bottleneck detection

### API support

* Better rewrite middleware with ability to rewrite request/response bodies
* IP blacklists/whitelists with pluggable backends
* Request HMAC signing/checking
* Dynamic rate-limiting support via rate-limit middleware


### Clustering

* Implementing Leader/Follower pattern, IP takeover
* Centralized metrics collection
* Rate limiting with pluggable backends
* Pluggable caching - Cassandra
* Integration with Kubernetes


