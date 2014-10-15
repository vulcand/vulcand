test: clean
	go test -v ./... -cover

deps:
	go list -f '{{join .Deps "\n"}} \
{{join .TestImports "\n"}}' ./... |  xargs go list -e -f '{{if not .Standard}}{{.ImportPath}}{{end}}' | grep -v `go list` | xargs go get -u -v

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
