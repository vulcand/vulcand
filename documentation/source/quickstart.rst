.. _quickstart:

Quick Start
===========

.. figure::  _static/img/Vulcan1.png
   :align:   left


Vulcand uses Etcd as a configuration backend. See Etcd `getting started guide <https://github.com/coreos/etcd#getting-started>`_ for instructions.
Once Etcd and Vulcand is running, set up

Example: setting up Vulcand
"""""""""""""""""""""""""""

.. code-block:: bash_etcd

 # Upsert upstream and add an endpoint to it
 etcdctl set /vulcand/upstreams/up1/endpoints/e1 http://localhost:5000

 # Upsert a host "localhost" and add a location to it that matches path "/home" and uses endpoints from upstream "up1"
 etcdctl set /vulcand/hosts/localhost/locations/loc1/path 'TrieRoute("/home")'
 etcdctl set /vulcand/hosts/localhost/locations/loc1/upstream up1


.. code-block:: bash_cli

 # Upsert upstream and add an endpoint to it
 etcdctl set /vulcand/upstreams/up1/endpoints/e1 http://localhost:5000

 # Upsert a host "localhost" and add a location to it that matches path "/home" and uses endpoints from upstream "up1"
 etcdctl set /vulcand/hosts/localhost/locations/loc1/path 'TrieRoute("/home")'
 etcdctl set /vulcand/hosts/localhost/locations/loc1/upstream up1

.. code-block:: bash_api

 # Upsert upstream and add an endpoint to it
 etcdctl set /vulcand/upstreams/up1/endpoints/e1 http://localhost:5000

 # Upsert a host "localhost" and add a location to it that matches path "/home" and uses endpoints from upstream "up1"
 etcdctl set /vulcand/hosts/localhost/locations/loc1/path 'TrieRoute("/home")'
 etcdctl set /vulcand/hosts/localhost/locations/loc1/upstream up1
