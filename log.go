/*
 * Copyright 2023 github.com/fatima-go
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * @project fatima-core
 * @author dave
 * @date 23. 4. 12. 오후 1:31
 */

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
	// 설정이 자리잡기 전에 끼어든 로그가 빈 설정으로 만든 소스 표기를 캐시에 남기지 않도록, 설정 반영과 캐시 리셋을 한 락 안에서 한다.
	callSiteMu.Lock()
	logPreference = pref
	normalizePreference(&logPreference)
	logPreference.logFilePath = fmt.Sprintf("%s.log", filepath.Join(pref.logFolder, pref.ProcessName))
	resetCallSites()
	callSiteMu.Unlock()
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

// SetSourcePrintSize 소스 표기 길이를 바꾼다. 캐시된 소스 표기가 달라지므로 값이 바뀌면 호출 위치 캐시를 비운다.
func SetSourcePrintSize(newValue uint8) {
	// minimum source print size : 10
	if newValue < 10 {
		return
	}

	callSiteMu.Lock()
	defer callSiteMu.Unlock()

	if logPreference.SourcePrintSize == newValue {
		return
	}

	logPreference.SourcePrintSize = newValue
	resetCallSites()
}

// SetShowMethod 메서드명 표기 여부를 바꾼다. 캐시된 소스 표기가 달라지므로 값이 바뀌면 호출 위치 캐시를 비운다.
func SetShowMethod(newValue bool) {
	callSiteMu.Lock()
	defer callSiteMu.Unlock()

	if logPreference.ShowMethod == newValue {
		return
	}

	logPreference.ShowMethod = newValue
	resetCallSites()
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

// print skip 은 runtime.Caller 기준으로 print 자신이 0, 로그 함수를 호출한 쪽이 2 다.
func print(skip int, level LogLevel, v ...interface{}) {
	// resolveCallSite 안의 runtime.Callers 기준으로는 Callers(0), resolveCallSite(1) 두 프레임이 더 있다.
	site, depth := resolveCallSite(skip + 2)

	var logEvent LogEvent

	if originError, ok := v[len(v)-1].(error); ok {
		errEvent := newErrorTraceLogEvent(site, originError)
		// 트레이스는 찾아낸 호출 위치의 바로 위 프레임부터 남긴다. 헬퍼를 거쳐도 시작점이 밀리지 않는다.
		if limit := int(logPreference.MaxErrorTraceLevel) - 3; limit > 0 {
			pcs := make([]uintptr, limit)
			n := runtime.Callers(skip+depth+2, pcs)
			frames := runtime.CallersFrames(pcs[:n])
			for more := n > 0; more; {
				var frame runtime.Frame
				frame, more = frames.Next()
				errEvent.append(TracePoint{funcName: functionNameOf(frame.Function), file: frame.File, line: frame.Line})
			}
		}
		logEvent = errEvent
	} else {
		logEvent = newGeneralLogEvent(site)
	}

	logEvent.setLevel(level)
	logEvent.setArgs(v...)

	if logPreference.DeliveryMode == DELIVERY_MODE_SYNC {
		writeLogEvent(logEvent)
	} else {
		logEventChannel <- logEvent
	}
}
