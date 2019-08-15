module github.com/vulcand/vulcand

go 1.12

require (
	github.com/armon/go-proxyproto v0.0.0-20160718231624-3daa90aec003
	github.com/bshuster-repo/logrus-logstash-hook v0.0.0-20170822102739-ebf008572634
	github.com/buger/goterm v0.0.0-20161103140809-cc3942e537b1
	github.com/codahale/hdrhistogram v0.0.0-20161010025455-3a0bb77429bd
	github.com/codegangsta/cli v1.20.0
	github.com/coreos/etcd v3.3.9+incompatible
	github.com/coreos/go-semver v0.2.0
	github.com/gogo/protobuf v1.1.1
	github.com/golang/protobuf v1.1.0
	github.com/gorilla/context v1.1.1
	github.com/gorilla/mux v0.0.0-20160920230813-757bef944d0f
	github.com/gorilla/websocket v1.4.0
	github.com/gravitational/trace v0.0.0-20190726142706-a535a178675f
	github.com/jonboulle/clockwork v0.1.0
	github.com/kr/pretty v0.1.0
	github.com/kr/text v0.1.0
	github.com/mailgun/metrics v0.0.0-20150124003306-2b3c4565aafd
	github.com/mailgun/minheap v0.0.0-20170619185613-3dbe6c6bf55f
	github.com/mailgun/multibuf v0.0.0-20150714184110-565402cd71fb
	github.com/mailgun/timetools v0.0.0-20170619190023-f3a7b8ffff47
	github.com/mailgun/ttlmap v0.0.0-20170619185759-c1c17f74874f
	github.com/opentracing/opentracing-go v1.1.0 // indirect
	github.com/pkg/errors v0.8.0
	github.com/sirupsen/logrus v1.4.2
	github.com/ugorji/go v1.1.1
	github.com/vulcand/oxy v0.0.0-20180707144047-21cae4f7b50b
	github.com/vulcand/predicate v1.1.0
	github.com/vulcand/route v0.0.0-20181101151700-58b44163b968
	golang.org/x/crypto v0.0.0-20190308221718-c2843e01d9a2
	golang.org/x/net v0.0.0-20190724013045-ca1201d0de80
	golang.org/x/sys v0.0.0-20190422165155-953cdadca894
	golang.org/x/text v0.3.0
	google.golang.org/genproto v0.0.0-20180731170733-daca94659cb5
	google.golang.org/grpc v1.14.0
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127
	gopkg.in/mgo.v2 v2.0.0-20180705113604-9856a29383ce
)

replace github.com/vulcand/oxy => /Users/thrawn/Development/oxy
