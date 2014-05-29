package runtime

import (
	"fmt"
	"io/ioutil"
	"os"

	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
)

// Write process' PID file at the provided path.
func WritePid(path string) error {
	pid := os.Getpid()
	log.Infof("Writing PID %v to %v", pid, path)
	return ioutil.WriteFile(path, []byte(fmt.Sprint(pid)), 0644)
}
