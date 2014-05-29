ETCD_NODES := http://127.0.0.1:4001,http://127.0.0.1:4002,http://127.0.0.1:4003

test: clean
	go test -v ./... -cover

test-with-etcd: clean
	VULCAND_ETCD_NODES=${ETCD_NODES} go test -v ./... -cover

clean:
	find . -name flymake_* -delete

test-package: clean
	VULCAND_ETCD_NODES=${ETCD_NODES} go test -v ./$(p)

cover-package: clean
	VULCAND_ETCD_NODES=${ETCD_NODES} go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

sloccount:
	 find . -name "*.go" -print0 | xargs -0 wc -l

install: clean
	go install github.com/mailgun/vulcand
	cd vulcanctl && $(MAKE) install && cd ..

run: install
	vulcand -etcd=http://127.0.0.1:4001 -etcdKey=/vulcand -readTimeout=10s -writeTimeout=10s

