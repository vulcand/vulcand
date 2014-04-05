etcdctl mkdir /vulcan

etcdctl mkdir /vulcan/hosts/localhost

etcdctl mk /vulcan/hosts/localhost/locations/loc1/path /hello
etcdctl mk /vulcan/hosts/localhost/locations/loc1/upstream u1

etcdctl mkdir /vulcan/upstreams/u1
etcdctl mk /vulcan/upstreams/u1/endpoints/e1 http://localhost:5000
etcdctl mk /vulcan/upstreams/u1/endpoints/e2 http://localhost:5001

etcdctl mk /vulcan/hosts/localhost/locations/loc2/path /hello2
etcdctl mk /vulcan/hosts/localhost/locations/loc2/upstream u2

etcdctl mkdir /vulcan/upstreams/u2
etcdctl mk /vulcan/upstreams/u2/endpoints/e1 http://localhost:5002
etcdctl mk /vulcan/upstreams/u2/endpoints/e2 http://localhost:5003
