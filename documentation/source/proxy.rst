.. _proxy:

Vulcand - Proxy
===============

.. warning::  Status: Hardening, testing, benchmarking. Not usable for production yet.

* Vulcand is HTTP proxy that uses Etcd as a configuration backend. 
* Changes to configuration take effect immediately without restarting the service.
* Supports pluggable middlewares
* Handy command line tool

Mailing list: https://groups.google.com/d/forum/vulcan-proxy

.. figure::  http://coreos.com/assets/images/media/vulcan-1-upstream.png
   :align:   center

   How Vulcand works

Glossary
--------

Host
~~~~

Incoming requests are matched by their hostname first. Hostname is defined by incoming ``Host`` header.
E.g. ``curl http://example.com/alice`` will be matched by the host ``example.com`` first.

Location
~~~~~~~~
Hosts contain one or several locations. Each location defines a path - simply a regular expression that will be matched against request's url.
Location contains link to an upstream and vulcand will use the endpoints from this upstream to serve the request.

.. figure::  http://coreos.com/assets/images/media/vulcan-diagram.png
   :align:   center

   Location ``loc1`` will serve the request ``curl http://example.com/search`` because it matches the path ``/search`` and host ``example.com``

Upstream
~~~~~~~~
Upstream is a collection of endpoints. Upstream can be assigned to multiple locations at the same time. 
This is convenient as sometimes one endpoint serves multiple purposes and locations.


Endpoint
~~~~~~~~
Endpoint is a final destination of the incoming request, each endpoint is defined by ``<schema>://<host>:<port>``, e.g. ``http://localhost:5000``

Middleware
~~~~~~~~~~

Vulcand supports pluggable middlewares. Middlewares can intercept or transform the request to any location. Examples of the supported middlewares are rate limits and connection limits.
You can add or remove middlewares using command line, API or directly via backends.

Etcd
----

Vulcan supports reading and updating configuration based on the changes in Etcd. 
Vulcans watch etcd prefix that is supplied when running an instance and configure themselves.

.. note::  All examples bellow assume that Vulcand is configured to listen on ``/vulcand`` prefix, which is a default prefix.


Upstreams and endpoints
~~~~~~~~~~~~~~~~~~~~~~~

Upserts upstream and adds an endpoint to it:

.. code-block:: sh

 etcdctl set /vulcand/upstreams/up1/endpoints/e1 http://localhost:5000

Hosts and locations
~~~~~~~~~~~~~~~~~~~

Adding locations and hosts:

.. code-block:: sh

 # Upsert a host "localhost" and add a location to it that matches path "/home" and uses endpoints from upstream "up1"
 etcdctl set /vulcand/hosts/localhost/locations/loc1/path "/home"
 etcdctl set /vulcand/hosts/localhost/locations/loc1/upstream up1

The best part is that you can update upstream using the same command. Let's add another upstream and switch traffic to it:

.. code-block:: sh

 # create a new upstream with endpoint http://localhost:5003
 etcdctl set /vulcand/upstreams/up2/endpoints/e3 http://localhost:5003

 # redirect the traffic of the location "loc1" to the endpoints of the upstream "up2"
 etcdctl set /vulcand/hosts/localhost/locations/loc1/upstream up2


Note that you can add and remove endpoints to the existing upstream, and vulcan will start redirecting the traffic to them automatically:

.. code-block:: sh

 # Add a new endpoint to the existing upstream
 etcdctl set /vulcand/upstreams/up1/endpoints/e2 http://localhost:5001


Limits
~~~~~~

Vulcan supports setting rate and connection limits.

.. note::  Notice the priority in the examples below -  middlewares with lower priorities will be executed earlier.

.. code-block:: sh

 # Update or set rate limit the request to location "loc1" to 1 request per second per client ip 
 # with bursts up to 3 requests per second.
 etcdctl set /vulcand/hosts/localhost/locations/loc1/middlewares/ratelimit/rl1 '{"Type": "ratelimit", "Middleware":{"Requests":1, "PeriodSeconds":1, "Burst":3, "Variable": "client.ip"}}'


.. code-block:: sh

 # Update or set the connection limit to 3 simultaneous connections per client ip at a time
 etcdctl set /vulcand/hosts/localhost/locations/loc1/middlewares/connlimit/rl1 '{"Type": "connlimit", "Middleware":{"Requests":1, "PeriodSeconds":1, "Burst":3, "Variable": "client.ip"}}'


Command line
------------

Vulcanctl is a command line tool that provides a convenient way to confugure Vulcand processes.

Status
~~~~~~

Displays the configuration and stats about the daemon

.. code-block:: sh

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


Host
~~~~

Adding or removing host

.. code-block:: sh

 # Add host with name 'example.com'
 $ vulcanctl host add --name example.com

 # Remove host with name 'example.com'
 $ vulcanctl host rm --name example.com


Upstream
~~~~~~~~

Add or remove upstreams

.. code-block:: sh

 # Add upstream  with id 'u1'
 $ vulcanctl upstream add --id u1

 # Adds upstream with auto generated id
 $ vulcanctl upstream add 

 # Remove upstream with id 'u1'
 $ vulcanctl upstream rm --id u1

 # "Drain" - wait till there are no more active connections from the endpoints of the upstream 'u1'
 # or timeout after 10 seconds if there are remaining connections
 vulcanctl upstream drain -id u1 -timeout 10


Endpoint
~~~~~~~~

Endpoint command adds or removed endpoints to the upstream.

.. code-block:: sh

 # add endpoint with id 'e2' and url 'http://localhost:5002' to upstream with id 'u1'
 $ vulcanctl endpoint add --id e1 --up u1 --url http://localhost:5000 

 # in case if id is omitted, etcd will auto generate it
 $ vulcanctl endpoint add --up u1 --url http://localhost:5001 

 # removed endpoint with id 'e1' from upstream 'u1'
 $ vulcanctl endpoint rm --up u1 --id e1 


Location
~~~~~~~~

Add or remove location to the host

.. code-block:: sh

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

Rate limit
~~~~~~~~~~

Rate add or removes rate limit restrictions on the location

.. code-block:: sh

 # limit access per client ip to 10 requests per second in 
 # location 'loc1' in host 'example.com'
 $ vulcanctl ratelimit add --variable client.ip --host example.com --loc loc1 --requests 10

 # limit access per custom http header value 'X-Account-Id' to 100 requests per second 
 # to location 'loc1' in host 'example.com'
 $ vulcanctl ratelimit add --variable request.header.X-Account-Id --host example.com --loc loc1 --requests 10

 # remove rate limit restriction with id 'r1' from host 'example.com' location 'loc1'
 $ vulcanctl ratelimit rm --id r1  --host example.com --loc 'loc1'

Connection limit
~~~~~~~~~~~~~~~~

Control simultaneous connections for a location.

.. code-block:: sh

 # limit access per client ip to 10 simultaneous connections for
 # location 'loc1' in host 'example.com'
 $ vulcanctl connlimit add --id c1 -host example.com -loc loc1 -connections 10

 # limit access per custom http header value 'X-Account-Id' to 100 simultaneous connections
 # to location 'loc1' in host 'example.com'
 $ vulcanctl connlimit add --variable request.header.X-Account-Id --host example.com --loc loc1 --connections 10

 # remove connection limit restriction with id 'c1' from host 'example.com' location 'loc1'
 $ vulcanctl connlimit rm --id c1  --host example.com --loc 'loc1'


HTTP API Reference
------------------

Vulcan's HTTP API is the best way to configure one or several instances of Vulcan at the same time.  
Essentially it's a tiny wrapper around the Etcd backend, that writes to the Etcd.
Multiple Vulcand instances listening to the same prefix would detect changes simultaneously and reload configuration.

Status
~~~~~~

Check status
++++++++++++

.. code-block:: url

     GET /v1/status

Returns: ``200 OK``

.. code-block:: json

 {
    "Status": "ok"
 }


Host
~~~~

Get hosts
+++++++++

.. code-block:: url

     GET /v1/hosts

Example response:

.. code-block:: json

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


Add host
++++++++

.. code-block:: url

    POST 'application/json' /v1/hosts

Add a host to the proxy.

.. container:: ptable

 ================= ==========================================================
 Parameter         Description
 ================= ==========================================================
 name              Hostname      
 ================= ==========================================================

Example responses:

.. code-block:: json

 {
   "Name": "localhost"
 }


Delete host
++++++++++++

.. code-block:: url

    DELETE /v1/hosts/<name>

Delete a host.

Upstream
~~~~~~~~

Get upstreams
+++++++++++++

.. code-block:: url

    GET /v1/upstreams

Retrieve the existing upstreams. Example response:

.. code-block:: json

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


Add upstream
++++++++++++

.. code-block:: url

    POST 'application/json' /v1/upstreams

Add upstream to the proxy.

.. container:: ptable

 ================= ==========================================================
 Parameter         Description
 ================= ==========================================================
 id                Optional upstream id, will be generated if omitted.
 ================= ==========================================================

Example response:

.. code-block:: json

 {"Id": "up1"}


Delete upstream
+++++++++++++++

.. code-block:: url

    DELETE /v1/upstreams/<id>


Drain connections
+++++++++++++++++

.. code-block:: url

    GET /v1/upstreams/drain?timeout=3

Wait till there are no more connections to any endpoints to the upstream.

.. container:: ptable

 ================= ==========================================================
 Parameter         Description
 ================= ==========================================================
 timeout           Timeout in form `1s` for the amount of seconds to wait before time out.
 ================= ==========================================================

Example response:

.. code-block:: json

 {
   "Connections": 0
 }


Endpoint
~~~~~~~~

Get endpoints
+++++++++++++

.. code-block:: url

    GET /v1/upstreams/<id>/endpoints

Retrieve the endpoints of the upstream. Example response:

.. code-block:: json

 {
   "Endpoints": [
     {
       "Id": "e1",
       "Url": "http://localhost:5000",
       "UpstreamId": "up1"
     }
   ]
 }

Get endpoint
++++++++++++

.. code-block:: url

    GET /v1/upstreams/<id>/endpoints/<endpoint-id>

Retrieve the particular endpoint with id ``endpoint-id``

Add endpoint
++++++++++++

.. code-block:: url

    POST /v1/upstreams/<id>/endpoints

Add endpoint to the upstream. 

.. container:: ptable

 ================= ==========================================================
 Parameter         Description
 ================= ==========================================================
 id                Optional endppint id, will be generated if omitted
 url               Required valid endpoint url
 ================= ==========================================================

Example response:

.. code-block:: json

 {
   "Id": "e4",
   "Url": "http://localhost:5004",
   "UpstreamId": "up1",
   "Stats": null
 }


Delete endpoint
+++++++++++++++

.. code-block:: url

    DELETE /v1/upstreams/<id>/endpoints/<endpoint-id>

Delete an endpoint.


Location
~~~~~~~~

Get locations
+++++++++++++

.. code-block:: url

    GET /v1/hosts/<hostname>/locations

Retrieve the locations of the host. Example response:

.. code-block:: json

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


Get location
++++++++++++

.. code-block:: url

    GET /v1/hosts/<hostname>/locations/<location-id>

Retrieve the particular location in the host ``hostname`` with id ``location-id``

.. code-block:: json

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


Add location
++++++++++++

.. code-block:: url

    POST 'application/json' /v1/hosts/<hostname>/locations

Add a location to the host. Params:

.. container:: ptable

 ================= ==========================================================
 Parameter         Description
 ================= ==========================================================
 Id                Optional location id, will be generated if omitted.
 Path              Required regular expression for path matchng
 Upstream.Id       Required id of the existing upstream
 Hostname          Required hostname
 ================= ==========================================================

Example response:

.. code-block:: json

 {
   "Id": "loc2",
   "Hostname": "localhost",
   "Path": "/home",
   "Upstream": {
     "Id": "up1"
   }
 }


Delete location
++++++++++++++++

.. code-block:: url

    DELETE /v1/hosts/<hostname>/locations/<location-id>

Delete a location.


Update location upstream
++++++++++++++++++++++++

.. code-block:: url

    PUT /v1/hosts/<hostname>/locations/<location-id>

Update location's upstream. Gracefully Redirects all the traffic to the endpoints of the new upstream.


.. container:: ptable

 ================= ==========================================================
 Parameter         Description
 ================= ==========================================================
 upstream          Required id of the existing upstream
 ================= ==========================================================


Rate limit
~~~~~~~~~~

Get rate limits
+++++++++++++++

.. code-block:: url

    GET /v1/hosts/<hostname>/locations/<location-id>/middlewares/ratelimit/<rate-id>

Retrieve the particular rate of location in the host ``hostname`` with id ``location-id`` and rate id ``rate-id``
Example response:

.. code-block:: json

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


Add rate limit
++++++++++++++

.. code-block:: url

    POST 'application/json' /v1/hosts/<hostname>/locations/limits/rates

Add a rate limit to the location, will take effect immediately.

.. code-block:: json

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

Json parameters explained:

.. container:: ptable

 ================= ==========================================================
 Parameter         Description
 ================= ==========================================================
 Id                Optional rate id, will be generated if omitted
 Requests          Required amount of allowed requests
 PeriodSeconds     Required period in seconds for counting the requests
 Burst             Required allowed burst of the requests (additional requests exceeding the rate)
 Variable          Variable for rate limiting e.g. `client.ip` or `request.header.My-Header`
 ================= ==========================================================


Delete a rate limit
+++++++++++++++++++

.. code-block:: url

    DELETE /v1/hosts/<hostname>/locations/<location-id>/limits/rates/<rate-id>

Deletes rate limit from the location.


Update a rate limit
+++++++++++++++++++

.. code-block:: url

    PUT /v1/hosts/<hostname>/locations/<location-id>/limits/rates/<rate-id>

Update location's rate limit. Takes effect immdediatelly. Example response

.. code-block:: json

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


Connection limit
~~~~~~~~~~~~~~~~

Get connection limits
+++++++++++++++++++++

.. code-block:: url

    GET /v1/hosts/<hostname>/locations/<location-id>/middlewares/connlimit/<conn-id>

Retrieve the particular connection limit of location in the host ``hostname`` with id ``location-id`` and connection limit id ``conn-id``. Example response:

.. code-block:: json

 {
   "Id": "cl1",
   "Priority": 0,
   "Type": "connlimit",
   "Middleware": {
     "Connections": 3,
     "Variable": "client.ip"
   }
 }

Add connection limit
++++++++++++++++++++

.. code-block:: url

    POST 'application/json' /v1/hosts/<hostname>/locations/limits/connections

Add a connection limit to the location, will take effect immediately. Example response:

.. code-block:: json

 {
   "Id": "cl1",
   "Priority": 0,
   "Type": "connlimit",
   "Middleware": {
     "Connections": 3,
     "Variable": "client.ip"
   }
 }

JSON parameters explained

.. container:: ptable

 ================= ==========================================================
 Parameter         Description
 ================= ==========================================================
 Id                Optional limit id, will be generated if omitted.|
 Connections       Required maximum amount of allowed simultaneous connections|
 Variable          Variable for limiting e.g. ``client.ip`` or ``request.header.My-Header``
 ================= ==========================================================


Delete connection limit
+++++++++++++++++++++++ 

.. code-block:: url

    DELETE /v1/hosts/<hostname>/locations/<location-id>/middlewares/connlimit/<conn-id>

Delete a connection limit from the location.

Update connection limit
+++++++++++++++++++++++

.. code-block:: url

    PUT /v1/hosts/<hostname>/locations/<location-id>/limits/connections/<conn-id>

Update location's connection limit. Takes effect immdediatelly.

.. code-block:: json

 {
   "Id": "cl1",
   "Priority": 0,
   "Type": "connlimit",
   "Middleware": {
     "Connections": 3,
     "Variable": "client.ip"
   }
 }


Installation
------------

Docker builds
~~~~~~~~~~~~~~

Here's how you build vulcan in Docker:

.. code-block:: sh

 docker build -t mailgun/vulcan .


Starting the daemon:

.. code-block:: sh

 docker run -p 8182:8182 -p 8181:8181 mailgun/vulcan /opt/vulcan/vulcand -apiInterface="0.0.0.0" -interface="0.0.0.0" --etcd=http://10.0.3.1:4001


Don't forget to map the ports and bind to the proper interfaces, otherwise vulcan won't be reachable from outside the container.

Using the vulcanctl from container:

.. code-block:: sh

 docker run mailgun/vulcan /opt/vulcan/vulcanctl status  --vulcan 'http://10.0.3.1:8182'


Make sure you've specified ``--vulcan`` flag to tell vulcanctl where the running vulcand is. We've used lxc bridge interface in the example above.


Docker trusted build
~~~~~~~~~~~~~~~~~~~~~

There's a trusted ``mailgun/vulcand`` build you can use, it's updated automagically.


Manual installation
~~~~~~~~~~~~~~~~~~~

.. note:: You have to install go>=1.2 and Etcd before installing vulcand:

Install: 

.. code-block:: sh

  make install
  make run
