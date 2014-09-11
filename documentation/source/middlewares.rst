.. _middlewares:

Middlewares
-----------

Vulcand allows you to compile in middlewares that can change, intercept or reject the request or even alter the response, 
and provides ``vulcanbundle`` command line tool to make it easy.


Middlewares
~~~~~~~~~~~
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

Example of a request that passes auth and limiting middlewares and hits the endpoint:

.. figure::  _static/img/middleware/full_cycle.png
   :align: center

Example of a request that is rejected by limiting middleware:

.. figure::  _static/img/middleware/rejected.png
   :align: center

In this case endpoint won't process the request.

Vulcanbundle
~~~~~~~~~~~~

This is a very simple command line tool that creates a new version of vulcand daemon by writing a new ``main.go`` that imports the ``vulcand`` packages and your plugins.

.. figure::  _static/img/middleware/vulcanbundle.png
   :align: center

Example - Auth middleware
*************************

In this example, we will write a new fully functional (but insecure) middleware that will require all requests to be authorized with Http basic auth.
For this demo, we've created ``auth`` package in our environment. Full source code is available at: http://github.com/mailgun/vulcand-auth


Describing your middleware
//////////////////////////

Vulcand looks for a special function ``GetSpec`` that returns all that it needs to know about this middleware.

.. code-block:: go

 import (
     "github.com/mailgun/vulcand/plugin"
 )

 const Type = "auth"

 func GetSpec() *plugin.MiddlewareSpec {
     return &plugin.MiddlewareSpec{
         Type:      Type,       // A short name for the middleware
         FromOther: FromOther,  // Tells vulcand how to rcreate middleware from another one (this is for deserialization)
         FromCli:   FromCli,    // Tells vulcand how to create middleware from command line tool
         CliFlags:  CliFlags(), // Vulcand will add this flags to middleware specific command line tool
     }
 }

We will get back to each one of this functions later, for now let's create a middleware struct itself.
Our Auth plugin is a struct that stores username and password.

.. note:: Your struct should contain only values expected in: http://golang.org/pkg/encoding/json/#Marshal

.. code-block:: go

 // Auth middleware
 type Auth struct {
     Password string
     Username string
 }


Handlers
////////

Let's define ``ProcessRequest`` and ``ProcessResponse`` that will be called on each request and response.

.. code-block:: go

 // This function will be called each time the request hits the location with this middleware activated
 func (a *Auth) ProcessRequest(r Request) (*http.Response, error) {
     auth, err := netutils.ParseAuthHeader(r.GetHttpRequest().Header.Get("Authorization"))
     // We want to reject the request, so we create and return ``Forbidden`` response
     if err != nil || a.Username != auth.Username || a.Password != auth.Password {
         return netutils.NewTextResponse(r.GetHttpRequest(), http.StatusForbidden, "Forbidden"), nil
     }
     // Return a pair ``nil, nil`` indicates that we let the request continue
     // to the next middleware in chain or the endpoint.
     return nil, nil
 }

 func (*Auth) ProcessResponse(r Request, a Attempt) {
 }

.. warning:: if your ``ProcessRequest`` returns error, the request will be terminated, and will be treated 
             by Vulcand as an internal error. Panicking is also outstanding and is not part of the contract.


Utility functions
//////////////////

Let's define some other important functions as the next step.

.. code-block:: go

 // This function is optional but handy, used to check input parameters when creating new middlewares
 func NewAuth(user, pass string) (*Auth, error) {
     if user == "" || pass == "" {
         return nil, fmt.Errorf("Username and password can not be empty")
     }
     return &Auth{Username: user, Password: pass}, nil
 }

 // This function is important, it's called by vulcand to create a new instance of the middleware and put it into the
 // middleware chain for the location. In our case we just return our existing instance. In more complex cases you
 // may want to return something else or construct a different object
 func (r *Auth) NewMiddleware() (middleware.Middleware, error) {
     return r, nil
 }

 // Very insecure :-)
 func (r *Auth) String() string {
     return fmt.Sprintf("username=%s, pass=%s", r.Username, r.Password)
 }


Constructors
////////////

As the final step, lets define the functions required by ``GetSpec``, these ones will be called by ``Vulcand`` when it will need to
create new auth middlewares:

.. code-block:: go

 // Will be called by Vulcand when backend or API will read the middleware from the serialized bytes.
 // It's important that the signature of the function will be exactly the same, otherwise Vulcand will
 // fail to register this middleware.
 // The first and the only parameter should be the struct itself, no pointers and other variables.
 // Function should return middleware interface and error in case if the parameters are wrong.
 func FromOther(a Auth) (plugin.Middleware, error) {
     return NewAuth(a.Username, a.Password)
 }


 // Constructs the middleware from the command line
 func FromCli(c *cli.Context) (plugin.Middleware, error) {
     return NewAuth(c.String("user"), c.String("pass"))
 }

 func CliFlags() []cli.Flag {
     return []cli.Flag{
         cli.StringFlag{"user, u", "", "Basic auth username"},
         cli.StringFlag{"pass, p", "", "Basic auth pass"},
     }
 }


Imports
///////

Let's take a look at all imports used in the http://github.com/mailgun/vulcand-auth

.. code-block:: go

 // Note that I import the versions bundled with vulcand. This approach makes our lives easier, 
 // as we'll use exactly the same versions used by vulcand. 
 // Kind of escaping dependency management troubles thanks to Godep.
 import (
     "fmt"
     "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
     "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/middleware"
     "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
     . "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
     "github.com/mailgun/vulcand/plugin"
     "net/http"
 )


Vulcand uses `Godep <https://github.com/tools/godep>`_ to manage it dependencies, we encourage you to read it's docs. 
That's why all imports to shared libraries are pointing to versions vendored with the version of vulcand you are using.

Getting it together
///////////////////

Let's create a folder in ``GOPATH`` environment that will be used for your version of ``Vulcand`` compiled with the new middleware. 
In our case it looks like this:

.. code-block:: bash
 
 /home/alex/goworld/src/github.com/mailgun/vulcand-bundle

In your case it would be something different. Now execute the vulcanbundle command, you should see something like this:

.. code-block:: bash

 $ vulcanbundle init --middleware=github.com/mailgun/vulcand-auth/auth
 INFO  Jun 15 05:10:55.015: [main.go:50] SUCCESS: bundle vulcand and vulcanctl completed


.. note:: ``--middleware=github.com/mailgun/vulcand-auth/auth`` flag tells the tool to include our auth middleware into bundle.

Let us check that it actually did something:

.. code-block:: bash

 $ ls
 main.go  registry  vulcanctl

What just happened? Vulcanbundle wrote a new version of ``vulcand`` and ``vulcanctl`` that have our ``auth`` middleware plugged in.
The final step would be to install the bundle. Let's ``cd`` to the vulcand-bundle dir and execute the build commands

.. code-block:: bash

 $ go build -o vulcand
 $ pushd vulcanctl/ && go build -o vulcanctl && popd

Congrats! Now you have your version of ``vulcand`` and ``vulcanctl`` Let's use it and try a couple of tricks with the new middleware
to ensure it actually works.

We need to start the new binary and configure the server first. (If you have it configured, just skip configuration steps below)

.. code-block:: bash

 # start the daemon
 $ ./vulcand

 # add host, location and upstream with endpoints via newly compiled command line tool
 $ ./vulcanctl/vulcanctl host add -name localhost
 $ ./vulcanctl/vulcanctl upstream add -id up1
 $ ./vulcanctl/vulcanctl endpoint add -up up1 -url http://localhost:5000
 $ ./vulcanctl/vulcanctl location add -id loc1 -up up1 -host localhost -path / 


Vulcanctl
/////////

Now to the fun part, you can configure the new authorization. Make sure that auth command is now available:

.. code-block:: bash

 $ ./vulcanctl/vulcanctl auth --help
 $ ./vulcanctl/vulcanctl auth add --help

Let's play with the new feature!

.. code-block:: bash

 # add the auth
 $ ./vulcanctl/vulcanctl auth add --loc loc1 -host localhost --user hello --pass world
 OK: auth[id=14, priority=1, username=hello, pass=world] added

 # try it out
 $ curl http://localhost:8181
 Forbidden

 # ok, here you go:
 $ curl http://hello:world@localhost:8181
 ok


Etcd
////

Note that you can use Etcd to configure the new middleware, ``vulcand`` will load it from json representation:

.. code-block:: bash

 $ etcdctl set /vulcand/hosts/localhost/locations/loc1/middlewares/auth/auth1 '{"Type": "auth", "Middleware":{"Username": "user", "Password": "secret1"}}'


Testing
///////

We were in a hurry trying to get it all working, didn't we? Now we can step back and cover it all by tests:

https://github.com/mailgun/vulcand-auth/blob/master/auth/auth_test.go


Maintenance
///////////

Note that vulcanbundle simply writes the new ``vulcanbundle`` for you, and does not try to get in your way of managing your environment.
However, here are a couple of tips if you feel like you need ones:

* Your new vulcand-bundle is just a go program like any other one out there. 
* This means that you can add it to your repo, and update periodicallly by calling ``vulcanbundle init`` with new params.
* You can use ``godep save -r ./...`` with new bundle as well to make sure your bundle's deps are always threre.







 






