//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
//
// @project fatima
// @author DeockJin Chung (jin.freestyle@gmail.com)
// @date 2017. 3. 6. PM 7:42
//

package log

import (
	"fmt"
	"path/filepath"
	"runtime"
	"time"
)

var writingLogEvent = false

var logEventChannel = make(chan LogEvent, 1024)

var effectiveLogLevel LogLevel = LOG_NONE

type loggingStatus uint8

var loggerStatus loggingStatus = LOGGING_STATUS_NOT_STARTED
var logPreference preference

func Initialize(pref preference) {
	if loggerStatus > LOGGING_STATUS_NOT_STARTED {
		return
	}

	loggerStatus = LOGGING_STATUS_RUNNING
	logPreference = pref
	normalizePreference(&logPreference)
	logPreference.logFilePath = fmt.Sprintf("%s.log", filepath.Join(pref.logFolder, pref.ProcessName))
	if logPreference.DeliveryMode == DELIVERY_MODE_ASYNC {
		go func() {
			for {
				logEvent := <-logEventChannel
				writingLogEvent = true
				writeLogEvent(logEvent)
				if len(logEventChannel) == 0 {
					writingLogEvent = false
				}
			}
		}()
	}
	SetLevel(logPreference.DefaultLogLevel)
}

func SetSourcePrintSize(newValue uint8) {
	// minimum source print size : 10
	if newValue < 10 {
		return
	}

	logPreference.SourcePrintSize = newValue
}

func SetShowMethod(newValue bool) {
	logPreference.ShowMethod = newValue
}

func SetKeepingFileDays(days uint16) {
	// minimum keeping file days : 2
	if days < 2 || logPreference.streamMode == STREAM_MODE_STDOUT {
		return
	}

	var old = logPreference.KeepingFileDays
	logPreference.KeepingFileDays = days
	if old != days {
		Info("logging backup days changed to %d", logPreference.KeepingFileDays)
		go func() {
			removeOldLogFiles()
		}()
	}
}

func SetFileSizeLimitMB(mb uint16) {
	// minimum file size limit mb : 1
	if mb < 1 || logPreference.streamMode == STREAM_MODE_STDOUT {
		return
	}

	logPreference.LogfileSizeLimitMB = mb
	Info("[Not yet support] logging file size limit to %d MB", logPreference.LogfileSizeLimitMB)
}

func SetSentryDsn(dsn string, tags map[string]string) {
	logPreference.sentryDsn = dsn
	logPreference.sentryTag = tags
}

func SetSentryFlushSecond(second int) {
	if second > 0 {
		logPreference.sentryFlushSecond = uint8(second)
	}
}

func SetSentryLogLevel(logLevel string) {
	logPreference.sentryLogLevel = ConvertStringToLogLevel(logLevel)
}

func SetLevel(level LogLevel) {
	effectiveLogLevel = level
}

func GetLevel() LogLevel {
	return effectiveLogLevel
}

func Close() error {
	if loggerStatus == LOGGING_STATUS_SHUTDOWN {
		return nil
	}

	if logPreference.DeliveryMode == DELIVERY_MODE_SYNC {
		return nil
	}

	for {
		if len(logEventChannel) == 0 && !writingLogEvent {
			loggerStatus = LOGGING_STATUS_SHUTDOWN
			return nil
		}
		time.Sleep(time.Millisecond * 1)
	}
}

type customLogger struct {
	level LogLevel
}

func NewCustomLogger(loglevel string) customLogger {
	logger := customLogger{}
	logger.level = ConvertStringToLogLevel(loglevel)
	return logger
}

func (c customLogger) Printf(format string, a ...interface{}) {
	if loggerStatus == LOGGING_STATUS_RUNNING && effectiveLogLevel >= c.level {
		var s []interface{}
		s = append(s, format)
		s = append(s, a...)
		print(3, c.level, s...)
	}
}

func IsErrorEnabled() bool {
	if loggerStatus == LOGGING_STATUS_RUNNING && effectiveLogLevel >= LOG_ERROR {
		return true
	}
	return false
}

func Error(v ...interface{}) {
	if loggerStatus == LOGGING_STATUS_RUNNING && effectiveLogLevel >= LOG_ERROR && len(v) > 0 {
		print(2, LOG_ERROR, v...)
	}
}

func IsWarnEnabled() bool {
	if loggerStatus == LOGGING_STATUS_RUNNING && effectiveLogLevel >= LOG_WARN {
		return true
	}
	return false
}

func Warn(v ...interface{}) {
	if loggerStatus == LOGGING_STATUS_RUNNING && effectiveLogLevel >= LOG_WARN && len(v) > 0 {
		print(2, LOG_WARN, v...)
	}
}

func IsInfoEnabled() bool {
	if loggerStatus == LOGGING_STATUS_RUNNING && effectiveLogLevel >= LOG_INFO {
		return true
	}
	return false
}

func Info(v ...interface{}) {
	if loggerStatus == LOGGING_STATUS_RUNNING && effectiveLogLevel >= LOG_INFO && len(v) > 0 {
		print(2, LOG_INFO, v...)
	}
}

func IsDebugEnabled() bool {
	if loggerStatus == LOGGING_STATUS_RUNNING && effectiveLogLevel >= LOG_DEBUG {
		return true
	}
	return false
}

func Debug(v ...interface{}) {
	if loggerStatus == LOGGING_STATUS_RUNNING && effectiveLogLevel >= LOG_DEBUG && len(v) > 0 {
		print(2, LOG_DEBUG, v...)
	}
}

func IsTraceEnabled() bool {
	if loggerStatus == LOGGING_STATUS_RUNNING && effectiveLogLevel >= LOG_TRACE {
		return true
	}
	return false
}

func Trace(v ...interface{}) {
	if loggerStatus == LOGGING_STATUS_RUNNING && effectiveLogLevel >= LOG_TRACE && len(v) > 0 {
		print(2, LOG_TRACE, v...)
	}
}

func print(skip int, level LogLevel, v ...interface{}) {
	pc, file, line, _ := runtime.Caller(skip)

	var logEvent LogEvent

	if originError, ok := v[len(v)-1].(error); ok {
		errEvent := newErrorTraceLogEvent(pc, file, line, originError)
		for i := 3; i < int(logPreference.MaxErrorTraceLevel); i++ {
			pc, file, line, exist := runtime.Caller(i)
			if !exist {
				break
			}
			point := TracePoint{pc: pc, file: file, line: line}
			errEvent.append(point)
		}
		logEvent = errEvent
	} else {
		logEvent = newGeneralLogEvent(pc, file, line)
	}

	logEvent.setLevel(level)
	logEvent.setArgs(v...)

	if logPreference.DeliveryMode == DELIVERY_MODE_SYNC {
		writeLogEvent(logEvent)
	} else {
		logEventChannel <- logEvent
	}
}
