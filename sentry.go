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
// @date 2020. 9. 24. PM 7:42
//

package log

import (
	"fmt"
	"github.com/getsentry/sentry-go"
)

const (
	tagEnvironment = "environment"
	tagServerName  = "serverName"
	tagProcess     = "process"
)

var sentryConnect = false

var (
	sentryError *sentry.Hub
	sentryWarn  *sentry.Hub
	sentryInfo  *sentry.Hub
)

func SentryInit() {
	Info("sentry initializing.. dsn=[%s], level=[%s]", logPreference.sentryDsn, logPreference.sentryLogLevel)
	if len(logPreference.sentryDsn) < 8 {
		fmt.Printf("discard invalid sentry dsn [%s]\n", logPreference.sentryDsn)
		return
	}

	// skip under info levelStr
	if logPreference.sentryLogLevel > LOG_INFO {
		fmt.Printf("discard sentry level over INFO\n")
		return
	}

	var environment string
	var serverName string
	var process string

	if logPreference.sentryTag != nil {
		environment, _ = logPreference.sentryTag[tagEnvironment]
		serverName, _ = logPreference.sentryTag[tagServerName]
		process, _ = logPreference.sentryTag[tagProcess]
	}

	err := sentry.Init(sentry.ClientOptions{
		// Either set your DSN here or set the SENTRY_DSN environment variable.
		Dsn: logPreference.sentryDsn,
		// Enable printing of SDK debug messages.
		// Useful when getting started or trying to figure something out.
		Debug:       false,
		Environment: environment,
		ServerName:  serverName,
	})

	if err != nil {
		fmt.Printf("fail to init sentry : %s", err.Error())
		return
	}

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("process", process)
	})

	if logPreference.sentryLogLevel >= LOG_INFO {
		sentryInfo = sentry.CurrentHub().Clone()
		sentryInfo.Scope().SetLevel(sentry.LevelInfo)
	}
	if logPreference.sentryLogLevel >= LOG_WARN {
		sentryWarn = sentry.CurrentHub().Clone()
		sentryWarn.Scope().SetLevel(sentry.LevelWarning)
	}
	if logPreference.sentryLogLevel >= LOG_ERROR {
		sentryError = sentry.CurrentHub().Clone()
		sentryError.Scope().SetLevel(sentry.LevelError)
	}

	sentryConnect = true
}

func sentrySendMessage(level LogLevel, message string) {
	hub := getSentryHub(level)
	if hub != nil {
		hub.CaptureMessage(message)
	}
}

func sentrySendException(level LogLevel, err error) {
	hub := getSentryHub(level)
	if hub != nil {
		hub.CaptureException(err)
	}
}

func getSentryHub(level LogLevel) *sentry.Hub {
	if !sentryConnect {
		return nil
	}

	switch level {
	case LOG_ERROR:
		return sentryError
	case LOG_WARN:
		return sentryWarn
	case LOG_INFO:
		return sentryInfo
	}

	return nil
}
