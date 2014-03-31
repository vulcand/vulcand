etcdctl mkdir /vulcan
etcdctl mkdir /vulcan/servers/localhost
etcdctl mk /vulcan/servers/localhost/locations/loc1/path /hello
etcdctl mk /vulcan/servers/localhost/locations/loc1/endpoints/e1 http://localhost:5000
etcdctl mk /vulcan/servers/localhost/locations/loc1/endpoints/e2 http://localhost:5001

