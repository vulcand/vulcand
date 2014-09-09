ETCD_NODE1 := http://127.0.0.1:4001
ETCD_NODE2 := http://127.0.0.1:4002
ETCD_NODE3 := http://127.0.0.1:4003
ETCD_NODES := ${ETCD_NODE1},${ETCD_NODE2},${ETCD_NODE3}
API_URL := http://localhost:8182
SERVICE_URL := http://localhost:8181
PREFIX := /vulcandtest
BOX_KEY := 1b727a055500edd9ab826840ce9428dc8bace1c04addc67bbac6b096e25ede4b

ETCD_FLAGS := VULCAND_TEST_ETCD_NODES=${ETCD_NODES}
VULCAN_FLAGS := VULCAND_TEST_ETCD_NODES=${ETCD_NODES} VULCAND_TEST_ETCD_PREFIX=${PREFIX} VULCAND_TEST_API_URL=${API_URL} VULCAND_TEST_SERVICE_URL=${SERVICE_URL}

test: clean
	go test -v ./... -cover

test-with-etcd: clean
	${ETCD_FLAGS} go test -v ./... -cover

test-with-vulcan: clean
	${VULCAN_FLAGS} go test -v ./... -cover

clean:
	find . -name flymake_* -delete

test-package: clean
	go test -v ./$(p)

test-package-with-etcd: clean
	${ETCD_FLAGS} go test -v ./$(p)

test-package-with-vulcan: clean
	${VULCAN_FLAGS} go test -v ./$(p)

cover-package: clean
	go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

cover-package-with-etcd: clean
	${ETCD_FLAGS} go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

cover-package-with-vulcan: clean
	${VULCAN_FLAGS} go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

sloccount:
	 find . -path ./Godeps -prune -o -name "*.go" -print0 | xargs -0 wc -l

install: clean
	go install github.com/mailgun/vulcand
	cd vulcanctl && $(MAKE) install && cd ..

run: install
	vulcand -etcd=${ETCD_NODE1} -etcd=${ETCD_NODE2} -etcd=${ETCD_NODE3} -etcdKey=/vulcand -sealKey=${BOX_KEY}

run-test-mode: install
	vulcand -etcd=${ETCD_NODE1} -etcd=${ETCD_NODE2} -etcd=${ETCD_NODE3} -etcdKey=${PREFIX} -sealKey=${BOX_KEY}

