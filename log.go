package scp

import (
	"log"
	"os"
)

func (r *Option) log(msg string, args ...interface{}) {
	if r.Logger == nil {
		return
	}
	r.Logger.Printf(msg, args...)
}

func (r *protocol) log(msg string, args ...interface{}) {
	if r == nil || r.opt == nil {
		return
	}
	r.opt.log(msg, args...)
}

type Logger interface {
	Printf(msg string, args ...interface{})
}

func NewFileLogger(file string) (Logger, error) {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o664)
	if err != nil {
		return nil, err
	}

	return &fileLogger{f: log.New(f, "[scp] ", log.LstdFlags)}, nil
}

type fileLogger struct {
	f *log.Logger
}

func (r *fileLogger) Printf(msg string, args ...interface{}) {
	r.f.Printf(msg, args...)
}
