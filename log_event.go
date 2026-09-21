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
	"time"
)

func newGeneralLogEvent(site *callSite) *GeneralLogEvent {
	event := GeneralLogEvent{}
	event.t = time.Now()
	event.site = site
	return &event
}

type GeneralLogEvent struct {
	t         time.Time
	level     LogLevel
	levelStr  string
	site      *callSite
	message   []interface{}
	published string
}

func (this *GeneralLogEvent) getMessage() string {
	return this.published
}

func (this *GeneralLogEvent) publish() {
	var express string
	if format, ok := this.message[0].(string); ok {
		express = fmt.Sprintf(format, this.message[1:]...)
	} else {
		express = fmt.Sprintf("%v", this.message[0])
	}

	this.published = this.buildMessage(func() string {
		return express
	})

	sentrySendMessage(this.level, express)
}

func (this *GeneralLogEvent) buildMessage(f func() string) string {
	return fmt.Sprintf("%s %s [%s] %s\n",
		this.t.Format("2006-01-02 15:04:05.000"),
		this.levelStr,
		this.site.source,
		f())
}

func (this *GeneralLogEvent) getTime() time.Time {
	return this.t
}

func (this *GeneralLogEvent) setLevel(level LogLevel) {
	this.level = level
	switch level {
	case LOG_ERROR:
		this.levelStr = "ERROR"
	case LOG_WARN:
		this.levelStr = "WARN "
	case LOG_INFO:
		this.levelStr = "INFO "
	case LOG_DEBUG:
		this.levelStr = "DEBUG"
	case LOG_TRACE:
		this.levelStr = "TRACE"
	}
}

func (this *GeneralLogEvent) setArgs(args ...interface{}) {
	this.message = args
}
