test: clean
	go test -v ./... -cover

deps:
	go get -v -u github.com/mailgun/vulcan
	go get -v -u github.com/coreos/go-etcd/etcd
	go get -v -u launchpad.net/gocheck

clean:
	find . -name flymake_* -delete

test-package: clean
	go test -v ./$(p) -cover

cover-package: clean
	go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

sloccount:
	 find . -name "*.go" -print0 | xargs -0 wc -l

install: clean
	go install github.com/mailgun/vulcand
	cd vulcanctl && $(MAKE) install && cd ..

run: install
	vulcand -etcd=http://127.0.0.1:4001 -etcdKey=/vulcan
