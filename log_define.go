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
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// logging status
const (
	LOGGING_STATUS_NOT_STARTED = 1 << iota
	LOGGING_STATUS_RUNNING
	LOGGING_STATUS_SHUTDOWN
)

// log levels
const (
	LOG_NONE  = 0x0  // 0000 0000
	LOG_ERROR = 0x7  // 0000 0111
	LOG_WARN  = 0xF  // 0000 1111
	LOG_INFO  = 0x1F // 0001 1111
	LOG_DEBUG = 0x2F // 0010 1111
	LOG_TRACE = 0xFF // 1111 1111
)

// logging preference default values
const (
	DEFAULT_KEEPING_FILE_DAYS   = 90
	DEFAULT_SOURCE_PRINT_SIZE   = 30
	DEFAULT_ERROR_TRACE_LEVEL   = 10
	DEFAULT_SENTRY_FLUSH_SECOND = 2
)

// logging preference delivery mode
const (
	DELIVERY_MODE_SYNC = 1 << iota
	DELIVERY_MODE_ASYNC
)

type LogDeliveryMode uint8

const (
	STREAM_MODE_STDOUT = 1 << iota
	STREAM_MODE_FILE
)

type LogStreamMode uint8

// log event
type LogEvent interface {
	getTime() time.Time
	getMessage() string
	setLevel(level LogLevel)
	setArgs(args ...interface{})
	publish()
}

// log levelStr type
type LogLevel uint8

func (this LogLevel) String() string {
	switch this {
	case LOG_DEBUG:
		return "DEBUG"
	case LOG_INFO:
		return "INFO"
	case LOG_TRACE:
		return "TRACE"
	case LOG_WARN:
		return "WARN"
	case LOG_ERROR:
		return "ERROR"
	}
	return "LOG_NONE"
}

func ConvertHexaToLogLevel(value string) (LogLevel, error) {
	if len(value) < 3 || (value[1] != 'x' && value[1] != 'X') {
		return LOG_NONE, errors.New("invalid value format")
	}
	parsed, err := strconv.ParseInt(value[2:], 16, 64)
	if err != nil {
		return LOG_NONE, err
	}

	switch parsed {
	case LOG_ERROR:
		return LOG_ERROR, nil
	case LOG_WARN:
		return LOG_WARN, nil
	case LOG_INFO:
		return LOG_INFO, nil
	case LOG_DEBUG:
		return LOG_DEBUG, nil
	case LOG_TRACE:
		return LOG_TRACE, nil
	}
	return LOG_NONE, nil
}

func ConvertStringToLogLevel(value string) LogLevel {
	switch strings.ToLower(value) {
	case "error":
		return LOG_ERROR
	case "warn":
		return LOG_WARN
	case "info":
		return LOG_INFO
	case "debug":
		return LOG_DEBUG
	case "trace":
		return LOG_TRACE
	}
	return LOG_NONE
}

func ConvertLogLevelToHexa(value string) string {
	if len(value) < 0 {
		return "0x0"
	}

	switch strings.ToLower(value) {
	case "info":
		return "0x1F"
	case "debug":
		return "0x2F"
	case "warn":
		return "0xF"
	case "error":
		return "0x7"
	case "trace":
		return "0xFF"
	}

	return "0x0"
}

// logging preference structure
type preference struct {
	logFolder          string
	streamMode         LogStreamMode
	ShowMethod         bool
	KeepingFileDays    uint16
	SourcePrintSize    uint8
	LogfileSizeLimitMB uint16
	MaxErrorTraceLevel uint8
	ProcessName        string
	sentryDsn          string
	sentryTag          map[string]string
	sentryFlushSecond  uint8
	sentryLogLevel     LogLevel
	DefaultLogLevel    LogLevel
	DeliveryMode       LogDeliveryMode
	logFileLoaded      bool
	logFilePath        string
	currentLogFileTime time.Time
	logFilePtr         *os.File
}

func NewPreference(logFolder string) preference {
	pref := createDefaultPreference(logFolder)
	pref.ProcessName = getProgramName()
	return pref
}

func NewPreferenceWithProcName(logFolder string, procName string) preference {
	pref := createDefaultPreference(logFolder)
	pref.ProcessName = procName
	return pref
}

func createDefaultPreference(logFolder string) preference {
	pref := preference{}

	if len(logFolder) < 1 {
		pref.streamMode = STREAM_MODE_STDOUT
	} else {
		err := ensureDirectory(logFolder)
		if err != nil {
			fmt.Printf("fail to prepare log folder : %s\n", err.Error())
			pref.streamMode = STREAM_MODE_STDOUT
		} else {
			pref.streamMode = STREAM_MODE_FILE
		}
	}

	pref.logFolder = logFolder
	pref.ShowMethod = true
	pref.DefaultLogLevel = LOG_TRACE
	pref.DeliveryMode = DELIVERY_MODE_SYNC
	pref.KeepingFileDays = DEFAULT_KEEPING_FILE_DAYS
	pref.SourcePrintSize = DEFAULT_SOURCE_PRINT_SIZE
	pref.MaxErrorTraceLevel = DEFAULT_ERROR_TRACE_LEVEL
	pref.sentryFlushSecond = DEFAULT_SENTRY_FLUSH_SECOND
	pref.sentryLogLevel = LOG_ERROR

	return pref
}

func normalizePreference(pref *preference) {
	if pref.KeepingFileDays < 1 {
		pref.KeepingFileDays = DEFAULT_KEEPING_FILE_DAYS
	}
	if pref.SourcePrintSize < 1 {
		pref.SourcePrintSize = DEFAULT_SOURCE_PRINT_SIZE
	}
	if pref.MaxErrorTraceLevel < 3 {
		pref.MaxErrorTraceLevel = DEFAULT_ERROR_TRACE_LEVEL
	}
}

func getProgramName() string {
	var procName string
	args0 := os.Args[0]
	lastIndex := strings.LastIndex(os.Args[0], "/")
	if lastIndex >= 0 {
		procName = args0[lastIndex+1:]
	} else {
		procName = os.Args[0]
	}

	firstIndex := strings.Index(procName, " ")
	if firstIndex > 0 {
		procName = procName[:firstIndex]
	}

	return procName
}

func ensureDirectory(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(path, 0755)
		}
	}

	return nil
}
