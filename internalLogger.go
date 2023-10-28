package fluent

import (
	"log"
	"os"
	"sync/atomic"
)

var internalLogger atomic.Value

func init() {
	internalLogger.Store(log.New(os.Stderr, "[fluent] ", log.LstdFlags))
}

// InternalLogger returns the Logger used to write out internal logs, where logs
// get written when something goes wrong in the logging stack itself.
func InternalLogger() *log.Logger { return internalLogger.Load().(*log.Logger) }

// SetInternalLogger makes l the internal logger. After this call, output from
// the log package's default Logger (as with [log.Print], etc.) will be logged
// at LevelInfo using l's Handler.
func SetInternalLogger(l *log.Logger) {
	internalLogger.Store(l)
}
