.. _quickstart:

Quick Start
===========

.. figure::  _static/img/Vulcan1.png
   :align:   left


Vulcand uses Etcd as a configuration backend. See Etcd `getting started guide <https://github.com/coreos/etcd#getting-started>`_ for instructions.

The easiest way to install Vulcand is to pull the trusted build from the hub.docker.com and launch it in the container:

.. code-block:: bash

  # download vulcand from the trusted build
  docker pull mailgun/vulcand

  # launch vulcand in a container
  docker run -d -p 8182:8182 -p 8181:8181 mailgun/vulcand /go/bin/vulcand -apiInterface=0.0.0.0 --etcd=http://172.17.42.1:4001

You can check if Vulcand is running by checking the logs of the container: 

.. code-block:: bash

  # check out the output from the container:
  docker logs $(docker ps | grep vulcand | awk '{ print $1 }')

  WARN Nov 13 20:44:21.263: PID:1 [supervisor.go:305] No hosts found

That was Vulcand complaining that there are no hosts specified. 

Example: setting up Vulcand
"""""""""""""""""""""""""""

.. code-block:: etcd

 # Upsert upstream and add an endpoint to it
 etcdctl set /vulcand/upstreams/up1/endpoints/e1 http://localhost:5000

 # Upsert a host "localhost" and add a location to it that matches path "/home" and uses endpoints from upstream "up1"
 etcdctl set /vulcand/hosts/localhost/locations/loc1/path 'TrieRoute("/home")'
 etcdctl set /vulcand/hosts/localhost/locations/loc1/upstream up1

.. code-block:: cli

 # Add upstream and endpoint
 vulcanctl upstream add -id up1
 vulcanctl endpoint add -id e1 -up up1 -url http://localhost:5000

 # Add host and location
 vulcanctl host add -name localhost
 vulcanctl location add -host=localhost -id=loc1 -up=up1 -path='TrieRoute("/home")'

.. code-block:: api

 #create upstream and endpoint
 curl -X POST -H "Content-Type: application/json" http://localhost:8182/v1/upstreams\
      -d '{"Id":"up1"}'
 curl -X POST -H "Content-Type: application/json" http://localhost:8182/v1/upstreams/up1/endpoints\
      -d '{"Id":"e1","Url":"http://localhost:5001","UpstreamId":"up1"}'

 # create host and a location
 curl -X POST -H "Content-Type: application/json" http://localhost:8182/v1/hosts\ 
      -d '{"Name":"localhost"}'
 curl -X POST -H "Content-Type: application/json" http://localhost:8182/v1/hosts/localhost/locations\
      -d '{"Hostname":"localhost","Id":"loc2","Upstream":{"Id":"up1"},"Path":"TrieRoute(\"/home\")"}'

 
