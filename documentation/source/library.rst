.. _library:

Vulcan - Reverse proxy library
==============================

Vulcan is a library for creating reverse http proxies written in Go.  If you are wondering, why would you need one, probably this doc is not for you, check out :ref:`proxy` docs instead.

Quickstart
----------

.. warning:: Vulcan is under heavy development, so be ready to see backwards-incompatible changes until the first stable release.

Here's a full functional proxy that adds two endpoints into RoundRobin load balancer:

.. code-block:: go

  package main

  import (
      "github.com/mailgun/vulcan"
      "github.com/mailgun/vulcan/endpoint"
      "github.com/mailgun/vulcan/loadbalance/roundrobin"
      "github.com/mailgun/vulcan/location/httploc"
      "github.com/mailgun/vulcan/route"
      "log"
      "net/http"
      "time"
  )

  func main() {

      // Create a round robin load balancer with some endpoints
      rr, err := roundrobin.NewRoundRobin()
      if err != nil {
          log.Fatalf("Error: %s", err)
      }
      rr.AddEndpoint(endpoint.MustParseUrl("http://localhost:5000"))
      rr.AddEndpoint(endpoint.MustParseUrl("http://localhost:5001"))

      // Create a http location with the load balancer we've just added
      loc, err := httploc.NewLocation("loc1", rr)
      if err != nil {
          log.Fatalf("Error: %s", err)
      }

      // Create a proxy server that routes all requests to "loc1"
      proxy, err := vulcan.NewProxy(&route.ConstRouter{Location: loc})
      if err != nil {
          log.Fatalf("Error: %s", err)
      }

      // Proxy acts as http handler:
      server := &http.Server{
          Addr:           "localhost:8181",
          Handler:        proxy,
          ReadTimeout:    10 * time.Second,
          WriteTimeout:   10 * time.Second,
          MaxHeaderBytes: 1 << 20,
      }
      server.ListenAndServe()

  }


Request
-------

Vulcan wraps each incoming http request to provide some features:

* ``GetId`` returns a unique sequential id assigned to each request, what is useful for debugging and logging.
* ``GetBody`` returns reader that performs disk buffering of large requests and multiple reads.
* Each attempt to forward the request is visible as ``Attempt`` entries.

.. code-block:: go

 package request

 type Request interface {
     GetHttpRequest() *http.Request
     GetId() int64
     GetBody() netutils.MultiReader
     AddAttempt(Attempt)
     GetAttempts() []Attempt
     GetLastAttempt() Attempt
     String() string
 }


Endpoint
--------

Endpoints define a final destination of the request. Each endpoint should provide a unique id, url and a human readable description.
Package ```endpoint``` provides utility functions constructing endpoints from http urls. Path of the url will be ommited and won't be used during request forwarding.

.. code-block:: go

 package endpoint

 type Endpoint interface {
     GetId() string
     GetUrl() *url.URL
     String() string
 }

Endpoint Examples
~~~~~~~~~~~~~~~~~

Construct endpoint from url:

.. code-block:: go

 import "github.com/mailgun/vulcan/endpoint"

 e, err := endpoint.ParseUrl("http://localhost:5000")


This one panics if url is incorrect:

.. code-block:: go

 import "github.com/mailgun/vulcan/endpoint"

 e := endpoint.MustParseUrl("http://localhost:5000")


Load balancer
-------------

Load balancers control the pool of endpoints, distribution of the requests, failover and failure detection. 

LoadBalancer interface provides a ``NextEndpoint`` method that will be called by a proxy before each request will be forwarded to a load balancer.

.. code-block:: go

 import "github.com/mailgun/vulcan/loadbalance"

 type LoadBalancer interface {
     NextEndpoint(req Request) (Endpoint, error)
     Middleware
     Observer
 }

Weighted round robin
~~~~~~~~~~~~~~~~~~~~

Vulcan library provides a weighted round robin load balancer (WRR) that comes with some batteries included:

* Failure detection
* Dynamic load balancing based on the failure rate

.. code-block:: go

 import "github.com/mailgun/vulcan/loadbalance/roundrobin"

 func NewBalancer() LoadBalancer {
     rr, err := roundrobin.NewRoundRobin()
     if err != nil {
         log.Fatalf("Error: %s", err)
     }
     rr.AddEndpoint(endpoint.MustParseUrl("http://localhost:5000"))
     rr.AddEndpoint(endpoint.MustParseUrl("http://localhost:5001"))
     return rr
 }

Some implementation details:

* WRR watches the failure rate using the in memory sliding window, 10 seconds by default with 1 second resolution.
* In case if some requests are failing, WRR tries to split endpoints in two groups: 'good' and 'bad' looking at their failure rates.
* If all the endpoints fail with similar error rates with insiginficant differences (e.g. 0.04 and 0.05) WRR does nothing.
* If there are some endpoints that have higher error rates comparing to others (e.g. 0.4 vs 0 or 0.06 vs 0.01) WRR tries to reduce the load on the 'bad' endpoints
by adjusting weights
* If adjusted weights did not make the situation worse (WRR identifies this by watching if the failure rates on 'good' endpoints increased) WRR commits the weights.
* This process continues till WRR reduces the load on 'bad' endpoints to a tiny portion of the overall traffic.


Locations
---------

Location is responsible for forwarding requests to a final destination and streaming back the response. 
Typically each service willl use it's own location, e.g. ``auth`` service will define its own location with a separate load balancer and endpoints.
Vulcan can work with one or multiple locations at the same time. 

.. code-block:: go

 package location

 type Location interface {
     GetId() string
     RoundTrip(Request) (*http.Response, error)
 }


HTTP location
~~~~~~~~~~~~~

Create http location with round robin load balancer:

.. code-block:: go

 import "github.com/mailgun/vulcan/httploc"

 location, err := httploc.NewLocation(roundrobin.NewRoundRobin())


Provide options to tune timeouts and failover policies:

.. code-block:: go

 import "github.com/mailgun/vulcan/httploc"

 location, err := httploc.NewLocationWithOptions(
           roundrobin.NewRoundRobin(), 
           httploc.Options{
              Timeouts: {Read: time.Second, Write: time.Second},
              ShouldFailover: failover.And(
                 failover.MaxAttempts(2), 
                 failover.OnErrors, 
                 failover.OnGets),
           })

HTTP location will round trip the HTTP request to a backend adding some headers with client information.


Router
------

Vulcan uses routers to match incoming request to a specific location and comes with a couple of routers for some common use-cases.

.. code-block:: go

 import "github.com/mailgun/vulcan/route"

 type Router interface {
     Route(req Request) (Location, error)
 }


Path router
~~~~~~~~~~~

Path router matches request URL's path against regular expression. It builds a single regular expression out of all expressions passed to it for efficient routing.

.. code-block:: go

 import "github.com/mailgun/vulcan/route/pathroute"

 router := pathroute.NewPathRouter()
 router.AddLocation("/auth", authLocation)
 router.AddLocation("/log", logsLocation)


Host router
~~~~~~~~~~~

This router composer helps to match request by hostname and uses inner routers to do further matching. 
This is useful in cases if one wants to implement classic Apache Vhosts routing, where each host defines independent routing rules.


.. code-block:: go

 import "github.com/mailgun/vulcan/route/hostroute"

 router := hostroute.NewHostRouter()
 router.SetRouter("www.example.com", websiteRouter)
 router.SetRouter("api.example.com", apiRouter)



.. include:: middlewares-intro.rst


Middleware
----------

Middlewares are allowed to observe, modify and intercept http requests and responses. Each middleware defines two methods, ``ProcessRequest`` and ``ProcessResponse``.

.. code-block:: go

 package middleware

 type Middleware interface {
     ProcessRequest(r Request) (*http.Response, error)
     ProcessResponse(r Request, a Attempt)
 }

* ``ProcessRequest`` is called before the request is going to be proxied to the endpoint selected by the load balancer. This function can modify or intercept request before it gets to a final destination.
* ``ProcessResponse`` is called after the response or error has been received from the final destination.



Middleware Chains
~~~~~~~~~~~~~~~~~

Middleware chains define an order in which middlewares will be executed. 
Each request passes through the sequence of middlewares calling ``ProcessRequest`` direct order. 
Once the request has been processed, response is passed through the same chain ``ProcessResponse`` in reverse order.

In case if middleware rejects the request, the request will be passed back through the middlewares that processed the request before.

Request passes auth and limiting middlewares
::
   | Request       | Response
   |               | 
 ┌─┼───────────────┼─┐
 │ │  Auth         │ │
 │ v               ^ │
 └─┼───────────────┼─┘
   │               │
 ┌─┼───────────────┼─┐
 │ │  Limiting     │ │ 
 │ v               ^ │ 
 └─┼───────────────┼─┘ 
   │               │   
 ┌─┼───────────────┼─┐
 │ │  Endpoint     │ │ 
 │ └────>───────>──┘ │ 
 └───────────────────┘ 


Request rejected by limiting middleware
::
   | Request       | Response
   |               | 
 ┌─┼───────────────┼─┐
 │ │  Auth         │ │ 
 │ v               ^ │ 
 └─┼───────────────┼─┘ 
   │               │   
 ┌─┼───────────────┼─┐
 │ │  Limiting     │ │ 
 │ └────>───────>──┘ │ 
 └───────────────────┘ 

 ┌───────────────────┐
 │    Caching        │ 
 │                   │ 
 └───────────────────┘ 

 ┌───────────────────┐
 │    Endpoint       │ 
 │                   │ 
 └───────────────────┘ 

In this case caching middleware and endpoint do not process the request.


Observers
---------

Unlinke middlewares, observers are not able to intercept or change any requests and will be called on every request to endpoint. 

Each location supports adding and removing observer to a chain. Observers are useful for metrics reporting, logging and other unobtrusive actions.

.. code-block:: go

 package middleware

 type Observer interface {
     ObserveRequest(r Request)
     ObserveResponse(r Request, a Attempt)
 }


Observers and middlewares call precedence
::
   | Request       | Response
   |               | 
 ┌─┼───────────────┼─┐
 │ │  Observer     │ │
 │ v  Chain        ^ │
 └─┼───────────────┼─┘
   │               │
 ┌─┼───────────────┼─┐
 │ │  Middleware   │ │ 
 │ v  Chain        ^ │ 
 └─┼───────────────┼─┘ 
   │               │   
 ┌─┼───────────────┼─┐
 │ │  Endpoint     │ │ 
 │ └────>───────>──┘ │ 
 └───────────────────┘ 

Precedence when middleware rejects request
::
   | Request       | Response
   |               | 
 ┌─┼───────────────┼─┐
 │ │  Observer     │ │ 
 │ v  Chain        ^ │ 
 └─┼───────────────┼─┘ 
   │               │   
 ┌─┼───────────────┼─┐
 │ │  Middlewares  │ │ 
 │ └────>───────>──┘ │ 
 └───────────────────┘ 

 ┌───────────────────┐
 │    Endpoint       │ 
 │                   │ 
 └───────────────────┘ 


Failover
--------

Failover forwards the request in case if backend failed to process the request. Each location can define it's own failover policy and there's no "one size fits all" approach and here's why:

Imagine you've set up a failover for all requests in case if backend did not respond to a request or dropped a connection. If your POST request activated some DB insert queries and hanged in the middle, the failover would trigger the same request and if you haven't used transactions or have been allocating some shared resources, that would happen again and again. That's why failover is usually safe when request is ``idempotent`` - can be repeated several times without errors.

Package ``failover`` provides some functions to construct a predicate that defines if this request should be retried

.. code-block:: go

 package failover

 type Predicate func(Request) bool

Failover on network errors only for get requests and limit the amount of attempts to 2:

.. code-block:: go

 import "github.com/mailgun/vulcan/failover"

 failover.And(
    failover.MaxAttempts(2), 
    failover.OnErrors, 
    failover.OnGets)


Limiter
-------

Limiters are implementations of the ``Middleware`` interface that ususally reject requests in case if clients exceed some rate or connection threshold.

.. code-block:: go

 package limit

 type Limiter interface {
     Middleware
 }


Mapper
~~~~~~

``MapperFn`` takes the request and returns token that will be limited, e.g. ``MapClientIp`` extracts client ip from the request, so the client ip will be rate limited.
One can define custom mappers to limit application specific properties, e.g. mapper returning account id from a request.

.. code-block:: go

 package limit

 type MapperFn func(r Request) (token string, amount int, err error)


Example of the client ip mapper

.. code-block:: go

 func MapClientIp(req Request) (string, int, error) {
     vals := strings.SplitN(req.GetHttpRequest().RemoteAddr, ":", 2)
     if len(vals[0]) == 0 {
         return "", -1, fmt.Errorf("Failed to parse client ip")
     }
     return vals[0], 1, nil
 }


Rate limiter
~~~~~~~~~~~~

Vulcan implements TokenBucket algorithm for a rate limiting that supports occasional controlled bursts but keeps the overall rate to a certain value.

.. code-block:: go

 import "github.com/mailgun/vulcan/limit/tokenbucket"

 limiter, err := tokenbucket.NewTokenLimiter(
          MapClientIp, Rate{Units: 1, Period: time.Second})


Limit client ip to 1 request per second with bursts up to 3 simultaneous requests:

.. code-block:: go

 import "github.com/mailgun/vulcan/limit/tokenbucket"

 l, err := NewTokenLimiterWithOptions(
         MapClientIp, Rate{Units: 1, Period: time.Second}, Options{Burst: 3})


Connection limiter
~~~~~~~~~~~~~~~~~~

Vulcan can limit the amount of simultaneous connections using `ConnectionLimiter`.

Limit the amount of simultaneous conections per IP to 10:

.. code-block:: go

 import "github.com/mailgun/vulcan/limit/connlimit"

 l, err := connlimit.NewConnectionLimiter(MapperClientIp, 10)

Metrics
-------

Vulcan watches the failure rate of the endpoint withing a moving time window by comparing the amount of successful requests to a number of failed requests. 
This metrics allows to activate failure recovery scenarios inside load balancers.

Calculates in memory failure rate of an endpoint

.. code-block:: go

 package metrics

 type FailRateGetter interface {
     GetRate() float64
     Observer
 }



