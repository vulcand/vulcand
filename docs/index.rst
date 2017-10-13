Documentation
-------------

Vulcand is a reverse proxy for HTTP API management and microservices. It is
 inspired by `Hystrix <https://github.com/Netflix/Hystrix>`_.

It uses Etcd as a configuration backend, so changes to configuration take
 effect immediately without restarting the service.

.. warning::  Status: Under active development. Used at Mailgun on moderate
 workloads.

.. toctree::
   :maxdepth: 2

   quickstart
   proxy
   middlewares
   api
