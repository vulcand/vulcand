.PHONY: all

PKGS := $(shell go list ./... | grep -v '/vendor/')

test: clean
	go test -v -race -cover $(PKGS)

dependencies:
	dep ensure -v

clean:
	find . -name flymake_* -delete

test-package: clean
	go test -v ./$(p)

test-grep-package: clean
	go test -v ./$(p) -check.f=$(e)

cover-package: clean
	go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

sloccount:
	 find . -path ./vendor -prune -o -name "*.go" -print0 | xargs -0 wc -l
