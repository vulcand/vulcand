.. _api:

API Reference
-------------

Vulcan's HTTP API is the best way to configure one or several instances of Vulcan at the same time.
Essentially it's a tiny wrapper around the backend. Multiple Vulcand instances listening to the same prefix would detect changes simultaneously and reload configuration.

Status
~~~~~~

Status endpoint is handy when you need to integrate Vulcand with another load balancer that can poll Vulcand to route the traffic based on the healthcheck response.

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


Add host listener
++++++++++++++++++

.. code-block:: url

     POST 'application/json' /v1/hosts/<name>/listeners

Add a location to the host. Listener parameters

.. code-block:: json

 {
   "Id": "", // id, will be auto-generated if omitted
   "Protocol": "https", // http or https
   "Address":
     {
        "Network":"tcp", // unix or tcp
        "Address":"localhost:8184"
     }
 }

Example response:

.. code-block:: json

 {
   "Id": "12", 
   "Protocol": "https", 
   "Address":
     {
        "Network":"tcp", 
        "Address":"localhost:8184"
     }
 }


Delete host listener
++++++++++++++++++++

.. code-block:: url

    DELETE /v1/hosts/<name>/listeners/<listener-id>

Delete a host listener


Set host certificate
++++++++++++++++++++

.. code-block:: url

     POST 'application/json' /v1/hosts/<name>/cert

Add a location to the host. Listener parameters

.. code-block:: json

 {
   "Key": "", // base64 encoded key string
   "Cert": "" // base64 encoded cert string
 }

Example response:

.. code-block:: json

 {
   "Key": "", // base64 encoded key string
   "Cert": "" // base64 encoded cert string
 }


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
 Options           Location options
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
