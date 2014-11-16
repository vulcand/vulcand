.. _proxy:

User Manual
===========


Glossary
--------

Familiarizing with the glossary would help to understand the rest of this guide.

Host
~~~~

Incoming requests are matched by their hostname first. Hostname is defined by incoming ``Host`` header.
E.g. ``curl http://example.com/alice`` will be matched by the host ``example.com`` first.

Listener
~~~~~~~~
Listener is a dynamic socket that can be attached or detached to host without restart. Host can have multiple http and https listeners 
attached to it, providing service on multiple interfaces and protocols.

Location
~~~~~~~~
Hosts contain one or several locations. Each location defines a path - simply a regular expression that will be matched against request's url.
Location contains link to an upstream and vulcand will use the endpoints from this upstream to serve the request.

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

Circuit Breaker
~~~~~~~~~~~~~~~
Circuit breakers are special type of middlewares that observe various metrics for a particular location and can activate failover scenario whenever the condition matches  e.g. error rate exceeds the threshold.

Secret storage
~~~~~~~~~~~~~~
Vulcand supports secret storage - running process acts like encryption/decryption service every time it reads and writes sensitive data, e.g. TLS certificates to the backend.
To use this feature, users generate ``sealKey`` using command line utility and pass this key to the process for encryption and decryption of the data in the backends.

Failover predicates
~~~~~~~~~~~~~~~~~~~

Sometimes it is handy to retry the request on error. The good question is what constitues an error? Sometimes it's a read/write timeout, and somethimes it's a special error code. 
Failover predicates are expressions that define when the request can be failed over, e.g.  ``IsNetworkError() && Attempts <= 2``

.. code-block:: bash

   IsNetworkError()         # failover on network error
   Attempts() <= 1          # allows only 1 failover attempt
   RequestMethod() == "GET" # failover for GET requests only
   ResponseCode() == 408    # failover on 408 HTTP response code

.. warning::  if you omit `Attempts`, failover will go into endless loop


Configuration
-------------

Vulcand can be configured via Etcd, API or command line tool - ``vulcanctl``. You can switch between different configuration examples using the samples switch.



Upstreams and endpoints
~~~~~~~~~~~~~~~~~~~~~~~

.. figure::  _static/img/VulcanUpstream.png
   :align:   left

Upstream is a collection of endpoints. Vulcand load-balances requests within the upstream and keeps the connection pool to every endpoint.
Locations using the same upstream will share the connections. 

Adding and removing endpoints to the used upstream will change the traffic in real-time, removing the upstream will lead to graceful drain off of the connections.

.. code-block:: etcd

 # Upsert upstream and add an endpoint to it
 etcdctl set /vulcand/upstreams/up1/endpoints/e1 http://localhost:5000


.. code-block:: cli

 # Add upstream and endpoint
 vulcanctl upstream add -id up1
 vulcanctl endpoint add -id e1 -up up1 -url http://localhost:5000


.. code-block:: api

 #create upstream and endpoint
 curl -X POST -H "Content-Type: application/json" http://localhost:8182/v1/upstreams\
      -d '{"Id":"up1"}'
 curl -X POST -H "Content-Type: application/json" http://localhost:8182/v1/upstreams/up1/endpoints\
      -d '{"Id":"e1","Url":"http://localhost:5001","UpstreamId":"up1"}'


**Upstream options**

Upstreams define the configuration options to the endpoints, such as the amount of idle connections and timeouts.
Upstream options are represented as JSON dictionary. 

.. code-block:: javascript

 {
   "Timeouts": {
      "Read":         "1s", // Socket read timeout (before we receive the first reply header)
      "Dial":         "2s", // Socket connect timeout
      "TlsHandshake": "3s", // TLS handshake timeout
   },
   "KeepAlive": {
      "Period":              "4s",  // Keepalive period for idle connections
      "MaxIdleConnsPerHost": 3,     // How many idle connections will be kept per host
   }
 }

One can update the options any time, that will initiate graceful reload of the underlying settings in Vulcand.

.. code-block:: etcd

 etcdctl set /vulcand/upstreams/u1/options '{"KeepAlive": {"MaxIdleConnsPerHost": 128, "Period": "4s"}}'

.. code-block:: cli

 vulcanctl upstream set_options -id up1 \
          -readTimeout=1s -dialTimeout=2s -handshakeTimeout=3s\
          -keepAlivePeriod=4s -maxIdleConns=128


.. code-block:: api

 curl -X PUT -H "Content-Type: application/json" http://localhost:8182/v1/upstreams/up1/options\
      -d '{"KeepAlive": {"MaxIdleConnsPerHost": 128, "Period": "4s"}}'


**Endpoint heartbeat**

Heartbeat allows to automatically de-register the endpoint when it crashes or wishes to be de-registered. 
Endpoint can heartbeat it's presense, and once the heartbeat is stopped, Vulcand will remove the endpoint from the rotation.

.. code-block:: bash

 # add  the endpoint to the upstream u1 for 5 seconds
 etcdctl set --ttl=5 /vulcand/upstreams/u1/endpoints/e1 http://localhost:5000



Hosts and locations
~~~~~~~~~~~~~~~~~~~

.. figure::  _static/img/VulcanLocation.png
   :align:   left


Request is first matched agains a host and finally redirected to a location. Location is matched by a path and optionally method.
It is recommended to specify a location per API method, e.g. ``TrieRoute("POST", "/v1/users")``.

Location needs a path and an existing upstream to start accepting requests.
You don't need to declare host explicitly, as it always a part of the location path, in this case it's ``localhost``

.. code-block:: etcd

 # add host and location
 etcdctl set /vulcand/hosts/localhost/locations/loc1/path 'TrieRoute("/home")'
 etcdctl set /vulcand/hosts/localhost/locations/loc1/upstream up1

.. code-block:: cli

 # add host and location
 vulcanctl host add -name localhost
 vulcanctl location add -host=localhost -id=loc1 -up=up1 -path='TrieRoute("/home")'

.. code-block:: api

 # add host and location
 curl -X POST -H "Content-Type: application/json" http://localhost:8182/v1/hosts\ 
      -d '{"Name":"localhost"}'
 curl -X POST -H "Content-Type: application/json" http://localhost:8182/v1/hosts/localhost/locations\
      -d '{"Hostname":"localhost","Id":"loc2","Upstream":{"Id":"up1"},"Path":"TrieRoute(\"/home\")"}'

**TLS Certificates**

Certificates are stored as encrypted JSON dictionaries. Updating a certificate will gracefully reload it for all running HTTP servers.

.. code-block:: etcd

 # Set keypair
 etcdctl set /vulcand/hosts/localhost/keypair '{...}'

.. code-block:: cli

 vulcanctl host set_keypair --privateKey=/path/key --cert=/path/cert

.. code-block:: api

 curl -X PUT -H "Content-Type: application/json" http://localhost:8182/v1/hosts/localhost/keypair\
      -d '{"Cert": "base64-encoded-certificate", "Key": "base64-encoded-key"}'

.. note:: When setting keypair via Etcd you need to encrypt keypair. This is explained in `TLS`_ section of this document.

**Location options**

Location options are represented as JSON dictionary. Location specifies various limits, forwarding and failover settings.

.. code-block:: javascript

 {
   "Limits": LocationLimits{
     "MaxMemBodyBytes": 12,  // Maximum request body size to keep in memory before buffering to disk
     "MaxBodyBytes": 400,    // Maximum request body size to allow for this location
   },
   "FailoverPredicate":  "IsNetworkError() && Attempts() <= 1", // Predicate that defines when requests are allowed to failover
   "Hostname":           "host1",                               // Host to set in forwarding headers
   "TrustForwardHeader": true,                                  // Time provider (useful for testing purposes)
 }

Setting location options upates the limits and parameters for the newly arriving requests in real-time.

.. code-block:: etcd

 etcdctl set /vulcand/hosts/localhost/locations/loc1/options\
         '{"FailoverPredicate":"(IsNetworkError() || ResponseCode() == 503) && Attempts() <= 2"}'

.. code-block:: cli

 vulcanctl location set_options\
         -host=localhost -id=loc1\
         -maxMemBodyKB=6 -maxBodyKB=7\
         -failoverPredicate='IsNetworkError()'\
         -trustForwardHeader\
         -forwardHost=host1

.. code-block:: api

 curl -X PUT -H "Content-Type: application/json" http://localhost:8182/v1/hosts/localhost/locations/loc1/options\
      -d '{"FailoverPredicate": "Attempts() <= 3"}'


**Switching upstreams**

Updating upstream gracefully re-routes the traffic to the new endpoints assigned to this upstream:

.. code-block:: etcd

 # redirect the traffic of the location "loc1" to the endpoints of the upstream "up2"
 etcdctl set /vulcand/hosts/localhost/locations/loc1/upstream up2

.. code-block:: cli

 # redirect the traffic of the location "loc1" to the endpoints of the upstream "up2"
 vulcanctl location set_upstream -host=localhost -id=loc1 -up=up2

.. code-block:: api

 # redirect the traffic of the location "loc1" to the endpoints of the upstream "up2"
 curl -X PUT http://localhost:8182/v1/hosts/localhost/locations/loc1 -F upstream=up2

.. note::  you can add and remove endpoints to the existing upstream, and vulcan will start redirecting the traffic to them automatically

Listeners
~~~~~~~~~
.. figure::  _static/img/VulcanListener.png
   :align:   left

Listeners allow attaching and detaching sockets on various interfaces and networks to multiple hosts. 
Hosts can have multiple listeners attached and share the same listener.

.. code-block:: javascript

 {
    "Protocol":"http",            // 'http' or 'https'
    "Address":{
       "Network":"tcp",           // 'tcp' or 'unix'
       "Address":"localhost:8183" // 'host:port' or '/path/to.socket'
    }
 }

.. code-block:: etcd

 # Add http listener accepting requests on 127.0.0.1:8183
 etcdctl set /vulcand/hosts/example.com/listeners/ls1\
            '{"Protocol":"http", "Address":{"Network":"tcp", "Address":"127.0.0.1:8183"}}'

.. code-block:: cli

 # Add http listener accepting requests on 127.0.0.1:80
 vulcanctl listener add --id ls1 --host example.com --proto=http --net=tcp -addr=127.0.0.1:8080


.. code-block:: api

 # Add http listener accepting requests on 127.0.0.1:8183
 curl -X POST -H "Content-Type: application/json" http://localhost:8182/v1/hosts/example.com/listeners\
      -d '{"Protocol":"http", "Address":{"Network":"tcp", "Address":"127.0.0.1:8183"}}'


Middlewares
~~~~~~~~~~~

.. figure::  _static/img/VulcanMiddleware.png
   :align:   left

Middlewares are allowed to observe, modify and intercept http requests and responses. Vulcand provides several middlewares. 
Users can write their own middlewares in Go.

To specify execution order of the middlewares, one can define the priority. Middlewares with smaller priority values will be executed first.

Rate Limits
~~~~~~~~~~~

Vulcan supports controlling request rates. Rate can be checked against different request parameters and is set up via limiting variable.

.. code-block:: bash
   
   client.ip                       # client ip
   request.header.X-Special-Header # request header

Adding and removing middlewares will modify the location behavior in real time. One can set expiring middlewares as well.

.. code-block:: etcd

 # Update or set rate limit the request to location "loc1" to 1 request per second per client ip 
 # with bursts up to 3 requests per second.
 etcdctl set /vulcand/hosts/localhost/locations/loc1/middlewares/ratelimit/rl1\
        '{"Priority": 0, "Type": "ratelimit", "Middleware":{"Requests":1, "PeriodSeconds":1, "Burst":3, "Variable": "client.ip"}}'


.. code-block:: cli

 # Update or set rate limit the request to location "loc1" to 1 request per second per client ip 
 # with bursts up to 3 requests per second.
 vulcanctl ratelimit add -id=rl1 -host=localhost -location=loc1 -requests=1 -burst=3 -period=1 --priority=0


.. code-block:: api

 # Update or set rate limit the request to location "loc1" to 1 request per second per client ip 
 # with bursts up to 3 requests per second.
 curl -X POST -H "Content-Type: application/json" http://localhost:8182/v1/hosts/localhost/locations/loc1/middlewares/ratelimit\
      -d '{"Priority": 0, "Type": "ratelimit", "Id": "rl1", "Middleware":{"Requests":1, "PeriodSeconds":1, "Burst":3, "Variable": "client.ip"}}'



Connection Limits
~~~~~~~~~~~~~~~~~

Connection limits control the amount of simultaneous connections per location. Locations re-use the same variables as rate limits.

.. code-block:: etcd

 # limit the amount of connections per location to 16 per client ip
 etcdctl set /vulcand/hosts/localhost/locations/loc1/middlewares/connlimit/cl1\
        '{"Priority": 0, "Type": "connlimit", "Middleware":{"Connections":16, "Variable": "client.ip"}}'


.. code-block:: cli

 # limit the amount of connections per location to 16 per client ip
 vulcanctl connlimit add -id=cl1 -host=localhost -location=loc1 -connections=1 --priority=0 --variable=client.ip


.. code-block:: api

 # limit the amount of connections per location to 16 per client ip
 curl -X POST -H "Content-Type: application/json" http://localhost:8182/v1/hosts/localhost/locations/loc1/middlewares/connlimit\
      -d '{"Priority": 0, "Type": "connlimit", "Middleware":{"Connections":16, "Variable": "client.ip"}}'



Circuit Breakers
~~~~~~~~~~~~~~~~

.. figure::  _static/img/CircuitStandby.png
   :align:   left

Circuit breaker is a special middleware that is designed to provide a fail-over action in case if service has degraded. 
It is very helpful to prevent cascading failures - where the failure of the one service leads to failure of another.
Circuit breaker observes requests statistics and checks the stats against special error condition.

.. figure::  _static/img/CircuitTripped.png
   :align:   left

In case if condition matches, CB activates the fallback scenario: returns the response code or redirects the request to another location. 

**Circuit Breaker states**

CB provides a set of explicit states and transitions explained below:

.. figure::  _static/img/CBFSM.png
   :align:   left

- Initial state is ``Standby``. CB observes the statistics and does not modify the request.
- In case if condition matches, CB enters ``Tripped`` state, where it responds with predefines code or redirects to another location.
- CB can execute the special HTTP callback when going from ``Standby`` to ``Tripped`` state
- CB sets a special timer that defines how long does it spend in the ``Tripped`` state
- Once ``Tripped`` timer expires, CB enters ``Recovering`` state and resets all stats
- In ``Recovering`` state Vulcand will start routing the portion of the traffic linearly increasing it over the period specified in ``Recovering`` timer.
- In case if the condition matches in ``Recovering`` state, CB enters ``Tripped`` state again
- In case if the condition does not match and recovery timer expries, CB enters ``Standby`` state.
- CB can execute the special HTTP callback when going from ``Recovering`` to ``Standby`` state


**Conditions**

CB defines a simple language that allows us to specify simple conditions that watch the stats for a location:

.. code-block:: javascript

 NetworkErrorRatio() > 0.5      // watch error ratio over 10 second sliding widndow for a location
 LatencyAtQuantileMS(50.0) > 50 // watch latency at quantile in milliseconds.
 ResponseCodeRatio(500, 600, 0, 600) > 0.5 // ratio of response codes in range [500-600) to  [0-600)

.. note::  Quantiles should be provided as floats - don't forget to add .0 to hint it as float

**Response fallback**

Response fallback will tell CB to reply with a predefined response instead of forwarding the request to the upstream

.. code-block:: javascript

 {
    "Type": "response", 
    "Action": {
       "ContentType": "text/plain",
       "StatusCode": 400, 
       "Body": "Come back later"
    }
 }

**Redirect fallback**

Redirect fallback will redirect the request to another location.

.. note::  It won't work for locations not defined in the Vulcand config.

.. code-block:: javascript

 {
    "Type": "redirect", 
    "Action": {
       "URL": "https://example.com/fallback"
    }
 }


**Webhook Action**

Circuit breaker can notify extenral sources on it's state transitions, e.g. it can create a pager duty incident by issuing a webhook:

.. code-block:: javascript

 {
  "Body": {
      "client": "Sample Monitoring Service",
      "client_url": "https://example.com",
      "description": "FAILURE for production/HTTP on machine srv01.acme.com",
      "event_type": "trigger",
      "incident_key": "srv01/HTTP",
      "service_key": "-pager-duty-service-key"
  },
  "Headers": {
      "Content-Type": [
          "application/json"
      ]
  },
  "Method": "POST",
  "URL": "https://events.pagerduty.com/generic/2010-04-15/create_event.json"
 }


**Setup**

Circuit breaker setup is can be done via Etcd, command line or API:

.. code-block:: etcd

 etcdctl set /vulcand/hosts/localhost/locations/loc1/middlewares/cbreaker/cb1 '{
              "Id":"cb1",
              "Priority":1,
              "Type":"cbreaker",
              "Middleware":{
                 "Condition":"NetworkErrorRatio() > 0.5",
                 "Fallback":{"Type": "response", "Action": {"StatusCode": 400, "Body": "Come back later"}},
                 "FallbackDuration": 10000000000,
                 "RecoveryDuration": 10000000000,
                 "CheckPeriod": 100000000
              }
            }'

.. code-block:: cli

 vulcanctl cbreaker add \
                   --host=localhost \
                   --location=loc1 \
                   --condition="NetworkErrorRatio() > 0.5" \
                   --fallback='{"Type": "response", "Action": {"StatusCode": 400, "Body": "Come back later"}}'


.. code-block:: api

 curl -X POST -H "Content-Type: application/json"\
      http://localhost:8182/v1/hosts/localhost/locations/loc1/middlewares/cbreaker\
      -d '{
              "Id":"cb1",
              "Priority":1,
              "Type":"cbreaker",
              "Middleware":{
                 "Condition":"NetworkErrorRatio() > 0.5",
                 "Fallback":"{\"Type\": \"response\", \"Action\": {\"StatusCode\": 400, \"Body\": \"Come back later\"}}",
                 "FallbackDuration": 10000000000,
                 "RecoveryDuration": 10000000000,
                 "CheckPeriod": 100000000
              }
            }'


TLS
---

Vulcand supports HTTPS via `SNI <http://en.wikipedia.org/wiki/Server_Name_Indication>`_, certificate management and multiple HTTPS instances. 
This sections below contain all the steps required to enable TLS support in Vulcand


Managing certificates
~~~~~~~~~~~~~~~~~~~~~

Vulcand encrypts certificates when storing them in the backends and uses `Nacl secretbox <https://godoc.org/code.google.com/p/go.crypto/nacl/secretbox>`_ to seal the data. 
The running server acts as an encryption/decryption point when reading and writing certificates.

This special key has to be generated by Vulcand using command line utility:

**Setting up seal key**

.. code-block:: bash 

 $ vulcanctl secret new_key

Once we got the key, we can pass it to the running daemon.

.. code-block:: bash

 $ vulcand -sealKey="the-seal-key"


.. note:: Add space before command to avoid leaking seal key in bash history, or use ``HISTIGNORE``

**Setting host keypair**

Setting certificate via etcd is slightly different from CLI and API:

.. code-block:: etcd

 # Read the private key and certificate and returns back the encrypted version that can be passed to etcd
 $ vulcanctl secret seal_keypair -sealKey <seal-key> -cert=</path-to/chain.crt> -privateKey=</path-to/key>

 # Once we got the certificate sealed, we can pass it to the Etcd:
 etcdctl set /vulcand/hosts/mailgun.com/keypair '{...}'

.. code-block:: cli

 # Connect to Vulcand Update the TLS certificate.
 # In this case we don't need to supply seal key, as in this case the CLI talks to the Vulcand directly
 $ vulcanctl host set_keypair -host <host> -cert=</path-to/chain.crt> -privateKey=</path-to/key>

.. code-block:: api

 # In this case we don't need to supply seal key, as in this case the CLI talks to the Vulcand directly
 curl -X PUT -H "Content-Type: application/json" http://localhost:8182/v1/hosts/localhost/keypair\
      -d '{"Cert": "base64-encoded-certificate", "Key": "base64-encoded-key-string"}'

.. note::  To update the certificate in the live mode just repeat the steps with the new certificate, vulcand will gracefully reload the config.


HTTPS listeners
~~~~~~~~~~~~~~~~

Once we have the certificate set, we can create HTTPS listeners for the host:

.. code-block:: etcd

 # Add http listener accepting requests on 127.0.0.1:8183
 etcdctl set /vulcand/hosts/example.com/listeners/ls1\
            '{"Protocol":"https", "Address":{"Network":"tcp", "Address":"127.0.0.1:8183"}}'

.. code-block:: cli

 # Add http listener accepting requests on 127.0.0.1:80
 vulcanctl listener add --id ls1 --host example.com --proto=https --net=tcp -addr=127.0.0.1:8080


.. code-block:: api

 # Add http listener accepting requests on 127.0.0.1:8183
 curl -X POST -H "Content-Type: application/json" http://localhost:8182/v1/hosts/example.com/listeners\
      -d '{"Protocol":"https", "Address":{"Network":"tcp", "Address":"127.0.0.1:8183"}}'


SNI
~~~

Not all clients support SNI, or sometimes host name is not available. In this case you can set the `default` certificate that will be returned in case if the SNI is not available:

.. code-block:: etcd

 # Set example.com as default host returned in case if SNI is not available
 etcdctl set /vulcand/hosts/example.com/options '{"Default": true}'


Metrics
--------

Metrics are provided for locations and endpoints:

.. code-block:: javascript

 {
   "Verdict":{
      "IsBad":false,    // Verdict will specify if there's something wrong with the endpoint
      "Anomalies":null  // Anomalies can be populated if Vulcand detects something unusual
   },
   "Counters":{             // Counters in a rolling time window
      "Period":10000000000, // Measuring period in ns
      "NetErrors":6,        // Network errors
      "Total":78,           // Total requests
      "StatusCodes":[
         {
            "Code":400,     // Status codes recorded
            "Count":7      
         },
         {
            "Code":429,
            "Count":67
         }
      ]
   },
   "LatencyBrackets":[ // Latency brackets recorded for the endpoint or location
      {
         "Quantile":99,
         "Value":172000  // microsecond resolution
      },
      {
         "Quantile":99.9,
         "Value":229000
      }
   ]
 }


Vulcand provides real-time metrics via API and command line.

.. code-block:: etcd

 # top acts like a standard linux top command, refreshing top active locations every second.
 vulcanctl top

.. code-block:: api

 # top locations
 curl http://localhost:8182/v1/hosts/top/locations?limit=100

 # top endpoints
 curl http://localhost:8182/v1/upstreams/top/endpoints?limit=100

.. code-block:: cli

 # top acts like a standard linux top command, refreshing top active locations every second.
 vulcanctl top

Logging
-------

Vulcand supports logging levels:

.. code-block:: bash
 
 INFO  # all output
 WARN  # warnings and errors only (default)
 ERROR # errors only

You can change the real time logging output by using ``set_severity`` command:

.. code-block:: etcd

  vulcanctl log set_severity -s=INFO
  
.. code-block:: api

  curl -X PUT http://localhost:8182/v1/log/severity -F severity=INFO

.. code-block:: cli

  # vulcanctl log set_severity -s=INFO

You can check current severity using ``get_severity`` command:

.. code-block:: etcd

  vulcanctl log get_severity
  
.. code-block:: api

  curl http://localhost:8182/v1/log/severity

.. code-block:: cli

  # vulcanctl log get_severity


Process management
------------------

Startup and configuration
~~~~~~~~~~~~~~~~~~~~~~~~~

Usage of vulcand

.. code-block:: sh

 vulcand
  
  -apiInterface="":              # apiInterface - interface for API
  -apiPort=8182                  # apiPort - port for API

  -etcd=[]                       # etcd - list of etcd discovery service API endpoints
  -etcdKey="vulcand"             # etceKey - etcd key for reading configuration

  -log="console"                 # log - syslog or console
  -logSeverity="WARN"            # log severity, INFO, WARN or ERROR
  -pidPath=""                    # path to write PID
  
  
  -sealKey=""                    # sealKey is used to store encrypted data in the backend,
                                 # use 'vulcanctl secret new_key' to create a new key.

  -statsdAddr="localhost:8185"   # statsdAddr - address where Vulcand will emit statsd metrics
  -statsdPrefix="vulcand"        # statsdPrefix is a prefix prepended to every metric

  -serverMaxHeaderBytes=1048576  # Maximum size of request headers in server


Binary upgrades
~~~~~~~~~~~~~~~

In case if you need to upgrade the binary on the fly, you can now use signals to reload the binary without downtime.

Here's how it works:

* Replace the binary with a new version
* Send ``USR2`` signal to a running vulcand instance 

.. code-block:: sh

  kill -USR2 $(pidof vulcand)

* Check that there are two instances running:

.. code-block:: sh

  4938 pts/12   Sl+    0:04 vulcand
  10459 pts/12   Sl+    0:01 vulcand

Parent vulcand process forks the child process and passes all listening sockets file descriptors to the child. 
Child process is now serving the requests along with parent process.

* Check the logs for errors

* If everything works smoothly, send ``SIGTERM`` to the parent process, so it will gracefully shut down:

.. code-block:: sh

  kill 4938

* On the other hand, if something went wrong, send ``SIGTERM`` to the child process and recover the old binary back.

.. code-block:: sh

  kill 10459

You can repeat this process multiple times.


Log control
~~~~~~~~~~~

You can controll logging verbosity by supplying ``logSeverity`` startup flag with the supported values ``INFO``, ``WARN`` and ``ERROR``, default value is ``WARN``.

If you need to temporarily change the logging for a running process (e.g. to debug some issue), you can do that by using ``set_severity`` command:

.. code-block:: sh

  vulcanctl log set_severity -s=INFO
  OK: Severity has been updated to INFO

You can check the current logging seveirty by using ``get_severity`` command:

.. code-block:: sh

  vulcanctl log get_severity
  OK: severity: INFO



Metrics
~~~~~~~

Vulcand can emit metrics to statsd via UDP. To turn this feature on, supply ``statsdAddr`` and ``statsdPrefix`` parameters to vulcand executable.

The service emits the following metrics for each location and endpoint:

+------------+-----------------------------------------------+
| Metric type| Metric Name                                   |
+============+===============================================+
| counter    | each distinct response code                   |
+------------+-----------------------------------------------+
| counter    | failure and success occurence                 |
+------------+-----------------------------------------------+
| gauge      | runtime stats (number of goroutines, memory)  |
+------------+-----------------------------------------------+



Installation
------------

Docker builds
~~~~~~~~~~~~~~

Here's how you build vulcan in Docker:

.. code-block:: sh

 docker build -t mailgun/vulcand .


Starting the daemon:

.. code-block:: sh

 docker run -d -p 8182:8182 -p 8181:8181 mailgun/vulcand /go/bin/vulcand -apiInterface="0.0.0.0" --etcd=http://172.17.42.1:4001


Don't forget to map the ports and bind to the proper interfaces, otherwise vulcan won't be reachable from outside the container.

Using the vulcanctl from container:

.. code-block:: sh

 docker run mailgun/vulcand /opt/vulcan/vulcanctl status  --vulcan 'http://172.17.42.1:8182'


Make sure you've specified ``--vulcan`` flag to tell vulcanctl where the running vulcand is. We've used lxc bridge interface in the example above.


Docker trusted build
~~~~~~~~~~~~~~~~~~~~~

There's a trusted ``mailgun/vulcand`` build you can use, it's updated automagically.


Manual installation
~~~~~~~~~~~~~~~~~~~

.. note:: You have to install go>=1.3 and Etcd before installing vulcand:

Install: 

.. code-block:: sh

  make install
  make run
