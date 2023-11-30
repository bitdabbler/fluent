package fluent

import (
	"log/slog"
	"time"
)

// HandlerOptions are used to customize the Fluent slog.Handler.
//
// NB: The struct pointer options approach is used to be consistent with the
// approach used in the standard library for `HandlerOptions`.
type HandlerOptions struct {

	// Level reports the minimum record level that will be logged. The handler
	// discards records with lower levels. If Level is nil, the handler assumes
	// LevelInfo. The handler calls Level.Level for each record processed; to
	// adjust the minimum level dynamically, use a LevelVar.
	Level slog.Leveler

	// WithTimeFormat configuration OptionFunc allows customization of the how time
	// values inside log content will get serialized. This does not change the
	// timestamp for the log itself, in the metadata, which is defined by the
	// Fluent protocol specification. The default is time.RFC3339Nano.
	TimeFormat string

	// AddSource causes the handler to compute the source code position of the
	// log statement and add a SourceKey attribute to the output.
	AddSource bool

	// Verbose controls whether debug logs are written to the internal logger.
	Verbose bool
}

const defaultTimeFormat = time.RFC3339Nano

// DefaultHandlerOptions returns *DefaultHandlerOptions with all default values.
func DefaultHandlerOptions() *HandlerOptions {
	return &HandlerOptions{
		Level:      slog.LevelInfo,
		TimeFormat: defaultTimeFormat,
	}
}

// resolve ensures that all options have valid values.
func (o *HandlerOptions) resolve() {

	// set default log level if not provided
	if o.Level == nil {
		o.Level = slog.LevelInfo
	}

	// set time format if missing, otherwise validate user provided one
	if len(o.TimeFormat) == 0 {
		o.TimeFormat = defaultTimeFormat
	} else {
		t := time.Now()
		s := t.Format(o.TimeFormat)
		_, err := time.Parse(s, o.TimeFormat)
		if err != nil {
			InternalLogger().Fatalf("HandlerOptions.TimeFormat is invalid: %v", err)
		}
	}
}
