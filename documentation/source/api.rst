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

     GET /v2/status

Returns: ``200 OK``

.. code-block:: json

 {
    "Status": "ok"
 }


Log severity
~~~~~~~~~~~~

Log severity endpoint allows to change the logging severity for a running instance

Get severity
++++++++++++

.. code-block:: url

     GET /v2/log/severity

Returns: ``200 OK``

.. code-block:: json

 {
    "Severity": "WARN"
 }

Set severity
++++++++++++

.. code-block:: url

     PUT 'multipart/form-data' /v1/log/severity

.. container:: ptable

 ================= ==========================================================
 Parameter         Description
 ================= ==========================================================
 severity          Severity - ``WARN``, ``INFO`` or ``ERROR``
 ================= ==========================================================

Returns: ``200 OK``

.. code-block:: json

 {
    "Message": "Severity has been updated to INFO"
 }


Host
~~~~

Get hosts
+++++++++

.. code-block:: url

     GET /v2/hosts

Example response:

.. code-block:: json

 {
   "Hosts":[
      {
         "Name":"localhost",
         "Settings":{
            "KeyPair":null,
            "Default":false
         }
      }
   ]
 }


Upsert host
++++++++++++

.. code-block:: url

    POST 'application/json' /v2/hosts

Add a host to the proxy.

.. code-block:: json

 {
 "Host": {
  "Name": "localhost",                              // hostname
  "Settings": {                                     // settings are optional
    "KeyPair": {"Cert": "base64", Key: "base64"},   // base64 encoded key-pair certficate
    "Default": false ,                              // default host for SNI
  }
 }
}


Example responses:

.. code-block:: json

 {
  "Name": "localhost",
  "Settings": {
    "KeyPair": null,
    "Default": false
  }
 }


Delete host
++++++++++++

.. code-block:: url

    DELETE /v2/hosts/<name>

Delete a host.


Listener
~~~~~~~~

Upsert listener
+++++++++++++++

.. code-block:: url

     POST 'application/json' /v2/listeners

Upsert listener

.. code-block:: json

 {
  "Listener": {
   "Id": "l1",
   "Protocol": "https", // http or https
   "Address":
     {
        "Network":"tcp", // unix or tcp
        "Address":"localhost:8184"
     }
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


Delete listener
++++++++++++++++++++

.. code-block:: url

    DELETE /v2/listeners/<listener-id>

Delete a listener


Backend
~~~~~~~~

Get backends
+++++++++++++

.. code-block:: url

    GET /v2/backends

Retrieve the existing upstreams. Example response:

.. code-block:: json

 {
  "Backends": [
    {
      "Id": "b1",
      "Type": "http",
      "Settings": {
        "Timeouts": {
          "Read": "",
          "Dial": "",
          "TLSHandshake": ""
        },
        "KeepAlive": {
          "Period": "",
          "MaxIdleConnsPerHost": 0
        }
      }
    }
  ]
 }


Upsert backend
++++++++++++++

.. code-block:: url

    POST 'application/json' /v2/backends


.. code-block:: json

 {
 "Backend": {
   "Id": "b1",
   "Type": "http",
   "Settings": {
     "Timeouts": {
       "Read": "5s",
       "Dial": "5s",
       "TLSHandshake": "10s"
     },
     "KeepAlive": {
       "Period": "30s",
       "MaxIdleConnsPerHost": 12
     }
   }
  }
 }

Example response:

.. code-block:: json

 {
  "Id": "b1",
  "Type": "http",
  "Settings": {
    "Timeouts": {
      "Read": "5s",
      "Dial": "5s",
      "TLSHandshake": "10s"
    },
    "KeepAlive": {
      "Period": "30s",
      "MaxIdleConnsPerHost": 12
    }
  }
 }


Delete backend
+++++++++++++++

.. code-block:: url

    DELETE /v2/backends/<id>


Server
~~~~~~

Get servers
+++++++++++++

.. code-block:: url

    GET /v2/backends/<id>/servers

Retrieve the servers of the backend. Example response:

.. code-block:: json

 {
  "Servers": [
    {
      "Id": "srv1",
      "URL": "http://localhost:5000"
    },
    {
      "Id": "srv2",
      "URL": "http://localhost:5003"
    }
  ]
 }

Get server
++++++++++++

.. code-block:: url

    GET /v2/backends/<id>/servers/<server-id>

Retrieve the particular server with id ``server-id``

Upsert endpoint
+++++++++++++++

.. code-block:: url

    POST /v1/upstreams/<id>/endpoints

Upsert server to the backend

.. code-block:: json

 {
  "Server": {
    "Id": "srv1",
    "URL": "http://localhost:5000"
  }
 }


Example response:

.. code-block:: json

 {
   "Id": "e4",
   "Url": "http://localhost:5004",
   "Stats": null
 }


Delete server
++++++++++++++

.. code-block:: url

    DELETE /v2/backends/<id>/servers/<server-id>

Delete a server.


Frontend
~~~~~~~~

Get frontends
+++++++++++++

.. code-block:: url

    GET /v2/frontends

Retrieve the frontends. Example response:

.. code-block:: json

 {
  "Frontends": [
    {
      "Id": "f1",
      "Route": "Path(`/`)",
      "Type": "http",
      "BackendId": "b1",
      "Settings": {
        "Limits": {
          "MaxMemBodyBytes": 0,
          "MaxBodyBytes": 0
        },
        "FailoverPredicate": "",
        "Hostname": "",
        "TrustForwardHeader": false
      }
    }
  ]
 }


Get frontend
++++++++++++

.. code-block:: url

    GET /v2/frontends/<frontend-id>

Retrieve the particular frontend with id ``frontend-id``

.. code-block:: json

 {
  "Id": "f1",
  "Route": "Path(`/`)",
  "Type": "http",
  "BackendId": "b1",
  "Settings": {
    "Limits": {
      "MaxMemBodyBytes": 0,
      "MaxBodyBytes": 0
    },
    "FailoverPredicate": "",
    "Hostname": "",
    "TrustForwardHeader": false
  }
 }


Upsert frontend
+++++++++++++++

.. code-block:: url

    POST 'application/json' /v1/hosts/<hostname>/frontends

Add a frontend to the host. Params:

.. code-block:: json

 {
  "Frontend": {
    "Id": "f1",
    "Route": "Path(`\/`)",
    "Type": "http",
    "BackendId": "b1",
    "Settings": {
      "Limits": {
        "MaxMemBodyBytes": 0,
        "MaxBodyBytes": 0
      },
      "FailoverPredicate": "",
      "Hostname": "",
      "TrustForwardHeader": false
    }
  }
 }


Example response:

.. code-block:: json

 {
  "Id": "f1",
  "Route": "Path(`/`)",
  "Type": "http",
  "BackendId": "b1",
  "Settings": {
    "Limits": {
      "MaxMemBodyBytes": 0,
      "MaxBodyBytes": 0
    },
    "FailoverPredicate": "",
    "Hostname": "",
    "TrustForwardHeader": false
  }
 }


Delete frontend
++++++++++++++++

.. code-block:: url

    DELETE /v2/frontends/<frontend-id>

Delete a frontend.


Rate limit
~~~~~~~~~~

Get rate limit
+++++++++++++++

.. code-block:: url

    GET /v2/frontends/<frontend-id>/middlewares/<middleware-id>

Retrieve the particular rate of frontend with id ``frontend-id`` and rate id ``rate-id``
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


Upsert rate limit
+++++++++++++++++

.. code-block:: url

    POST 'application/json' /v2/frontends/middlewares

Add a rate limit to the frontend, will take effect immediately.

.. code-block:: json

 {
  "Middleware": {
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

    DELETE /v2/frontends/<frontend-id>/middlewares/<middleware-id>

Deletes rate limit from the frontend.


Connection limit
~~~~~~~~~~~~~~~~

Get connection limit
++++++++++++++++++++

.. code-block:: url

    GET /v2/frontends/<frontend-id>/middlewares/<conn-id>

Retrieve the particular connection limit of frontend with id ``frontend-id`` and connection limit id ``conn-id``. Example response:

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

Upsert connection limit
+++++++++++++++++++++++

.. code-block:: url

    POST 'application/json' /v2/frontends/<frontend>/middlewares

Upsert a connection limit to the frontend. Example response:

.. code-block:: json

 {
  "Middleware": {
   "Id": "cl1",
   "Priority": 0,
   "Type": "connlimit",
   "Middleware": {
     "Connections": 3,
     "Variable": "client.ip"
   }
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

    DELETE /v2/frontends/<frontend-id>/middlewares/<conn-id>

Delete a connection limit from the frontend.
