.. _tls:

TLS
---

Vulcand support HTTPS via `SNI <http://en.wikipedia.org/wiki/Server_Name_Indication>`_, certificate management and multiple HTTPS instances. This guide contains all the steps required to enable TLS support in Vulcand


Managing certificates
~~~~~~~~~~~~~~~~~~~~~

Vulcand encrypts certificates when storing them in the backends and uses `Nacl secretbox <https://godoc.org/code.google.com/p/go.crypto/nacl/secretbox>`_ to seal the data. 
The running daemon acts as an encryption/decryption point when reading and writing certificates.

**Generating seal key**

To generate the key we can use ``vulcanctl`` tool:

.. code-block:: sh

 $ vulcanctl secret new_key

Once we got the key, we can pass it to the running daemon.

.. code-block:: sh

 $ vulcanctl vulcand -sealKey="the-seal-key"


**Setting host certiticate via ETCD**

First we need to seal the certificate and format it:

.. code-block:: sh

 # reads the private key and certificate and returns back the encrypted version that can be passed to etcd
 $ vulcanctl secret seal_cert -sealKey <seal-key> -cert=</path-to/chain.crt> -privateKey=</path-to/key>


Once we got the certificate sealed, we can pass it to the ETCD:

.. code-block:: sh

 # Set host certificate
 etcdctl set /vulcand/hosts/mailgun.com/cert '{...}'

.. note::  to update the certificate in the live mode just repeat the steps with the new certificate, vulcand will gracefully reload the config.


**Setting host certiticate via CLI**

Set certificate via command line tool:

.. code-block:: sh

 # Connect to Vulcand Update the TLS certificate.
 % vulcanctl host cet_cert -host 'example.com' -cert=</path-to/chain.crt> -privateKey=</path-to/key>

.. note::  in this case we don't need to supply seal key, as in this case the CLI talks to the Vulcand directly and Vulcand encrypts the cert for us


HTTPS listeners
~~~~~~~~~~~~~~~~

Once we have the certificate set, we can create HTTPS listeners for the host:

.. code-block:: sh

 # Add https listener accepting requests on localhpost:8184
 etcdctl set /vulcand/hosts/example.com/listeners/l1 '{"Protocol":"https", "Address":{"Network":"tcp", "Address":"localhost:8184"}}'

You can set one ore many listeners for the same host.

SNI
~~~

Not all clients support SNI, or sometimes host name is not available. In this case you can set the `default` certificate that will be returned in case if the SNI is not available:

.. code-block:: sh

 # Set example.com as default host returned in case if SNI is not available
 etcdctl set /vulcand/hosts/example.com/options '{"Default": true}'


