test: clean
	go test -v ./... -cover

deps:

	go get -v -u github.com/mailgun/gotools-log
	go get -v -u github.com/mailgun/gotools-time
	go get -v -u github.com/mailgun/ttlmap
	go get gopkg.in/check.v1

clean:
	find . -name flymake_* -delete

test-package: clean
	go test -v ./$(p)

bench-package: clean
	go test ./$(p) -check.bmem  -check.b -test.bench=.

cover-package: clean
	go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

sloccount:
	 find . -name "*.go" -print0 | xargs -0 wc -l
