etcdctl set /vulcand/upstreams/u1/endpoints/e1 http://localhost:5000
etcdctl set /vulcand/upstreams/u1/endpoints/e2 http://localhost:5001
etcdctl set /vulcand/upstreams/u1/endpoints/e3 http://localhost:5002
etcdctl set /vulcand/upstreams/u1/endpoints/e4 http://localhost:5003
etcdctl set /vulcand/upstreams/u1/endpoints/e5 http://localhost:5004

etcdctl set /vulcand/hosts/localhost/locations/loc1/path /
etcdctl set /vulcand/hosts/localhost/locations/loc1/upstream u1
