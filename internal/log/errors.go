package log

import "fmt"

type ErrUnknownLogFormat struct {
	err error
}

func (e ErrUnknownLogFormat) Error() string {
	return fmt.Sprintf("unknown log format: %v", e.err)
}

type ErrUnknownLogLevel struct {
	err error
}

func (e ErrUnknownLogLevel) Error() string {
	return fmt.Sprintf("unknown log level: %v", e.err)
}
