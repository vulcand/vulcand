etcdctl mkdir /vulcan

etcdctl mkdir /vulcan/servers/localhost

etcdctl mk /vulcan/servers/localhost/locations/loc1/path /hello
etcdctl mk /vulcan/servers/localhost/locations/loc1/upstream /vulcan/upstreams/u1

etcdctl mkdir /vulcan/upstreams/u1
etcdctl mk /vulcan/upstreams/u1/endpoints/e1 http://localhost:5000
etcdctl mk /vulcan/upstreams/u1/endpoints/e2 http://localhost:5001


etcdctl mk /vulcan/servers/localhost/locations/loc2/path /hello2
etcdctl mk /vulcan/servers/localhost/locations/loc2/upstream /vulcan/upstreams/u2

etcdctl mkdir /vulcan/upstreams/u2
etcdctl mk /vulcan/upstreams/u2/endpoints/e1 http://localhost:5002
etcdctl mk /vulcan/upstreams/u2/endpoints/e2 http://localhost:5003
