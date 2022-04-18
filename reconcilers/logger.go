/*
Copyright 2022 VMware, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package reconcilers

import (
	"fmt"
	"sync"

	"github.com/go-logr/logr"
)

var (
	_ logr.LogSink = (*warnOnceLogSink)(nil)
)

// Deprecated
func newWarnOnceLogger(log logr.Logger) logr.Logger {
	return logr.New(&warnOnceLogSink{
		sink: log.GetSink(),
	})
}

// Deprecated
type warnOnceLogSink struct {
	sink logr.LogSink
	once sync.Once
}

func (s *warnOnceLogSink) Init(info logr.RuntimeInfo) {
	s.sink.Init(info)
}

func (s *warnOnceLogSink) Enabled(level int) bool {
	return s.sink.Enabled(level)
}

func (s *warnOnceLogSink) Info(level int, msg string, keysAndValues ...interface{}) {
	s.warn()
	s.sink.Info(level, msg, keysAndValues...)
}

func (s *warnOnceLogSink) Error(err error, msg string, keysAndValues ...interface{}) {
	s.warn()
	s.sink.Error(err, msg, keysAndValues...)
}

func (s *warnOnceLogSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return &warnOnceLogSink{
		sink: s.sink.WithValues(keysAndValues...),
		once: s.once,
	}
}

func (s *warnOnceLogSink) WithName(name string) logr.LogSink {
	return &warnOnceLogSink{
		sink: s.sink.WithName(name),
		once: s.once,
	}
}

func (s *warnOnceLogSink) warn() {
	s.once.Do(func() {
		s.sink.Error(fmt.Errorf("Config.Log is deprecated"), "use a logger from the context: `log := logr.FromContext(ctx)`")
	})
}
