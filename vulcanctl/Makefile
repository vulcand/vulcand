VULCAN_API_URL := http://localhost:8182
VULCAN_SERVICE_URL := http://localhost:8181

test-with-vulcan: clean
	VULCAND_TEST_API_URL=${VULCAN_API_URL} VULCAND_TEST_SERVICE_URL=${VULCAN_SERVICE_URL} go test -v ./ -cover

cover-with-vulcan: clean
	VULCAND_TEST_API_URL=${VULCAN_API_URL} VULCAND_TEST_SERVICE_URL=${VULCAN_SERVICE_URL} go test -v ./ -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

test: clean
	go test -v ./ -cover

cover: clean
	go test -v ./ -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

deps:
	go get -v -u launchpad.net/gocheck
	go get -v -u github.com/codegangsta/cli
	go get -v -u github.com/wsxiaoys/terminal

clean:
	find . -name flymake_* -delete

sloccount:
	 find . -name "*.go" -print0 | xargs -0 wc -l

install: clean
	go install github.com/mailgun/vulcand/vulcanctl
