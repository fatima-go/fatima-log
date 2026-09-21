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
 * @date 26. 9. 21. 오전 9:18
 */

// Package calltest fatima-log 의 호출 위치 해석을 검증하려고 만든 테스트 전용 로깅 헬퍼다.
// 실제 애플리케이션의 로깅 헬퍼처럼 내부 호출 깊이가 메서드마다 다르도록 구성했다.
package calltest

import (
	log "github.com/fatima-go/fatima-log"
)

// Register 이 패키지를 로깅 헬퍼로 등록한다. 등록 전후 동작을 비교하려고 init() 대신 테스트가 직접 호출한다.
func Register() {
	log.RegisterHelperPackage()
}

// Trace 값 리시버와 포인터 리시버 메서드를 함께 가진 헬퍼다.
type Trace struct{}

// Info fatima-log 를 바로 호출한다(헬퍼 1단).
func (t Trace) Info(msg string) {
	log.Info("%s", msg)
}

// Finish Info 를 거친다(헬퍼 2단).
func (t Trace) Finish(msg string) {
	t.Info(msg)
}

// Done Finish 와 Info 를 거친다(헬퍼 3단).
func (t Trace) Done(msg string) {
	t.Finish(msg)
}

// Warn 포인터 리시버 메서드에서 로그를 남긴다.
func (t *Trace) Warn(msg string) {
	log.Warn("%s", msg)
}

// Fail 오류 트레이스가 붙는 로그를 남긴다.
func (t Trace) Fail(msg string, err error) {
	log.Error(msg, err)
}

// Closure 헬퍼 안의 클로저에서 로그를 남긴다.
func Closure(msg string) {
	func() {
		log.Info("%s", msg)
	}()
}

// Generic 제네릭 함수에서 로그를 남긴다.
func Generic[T any](value T) {
	log.Info("%v", value)
}

// Async 헬퍼 안에서 시작한 고루틴에서 로그를 남기고 끝날 때까지 기다린다. 이 경우 스택에 헬퍼 밖의 호출부가 없다.
func Async(msg string) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		log.Info("%s", msg)
	}()
	<-done
}

// Deep 헬퍼 프레임을 depth 개 더 쌓은 뒤 로그를 남긴다.
//
//go:noinline
func Deep(depth int, msg string) {
	if depth > 0 {
		Deep(depth-1, msg)
		return
	}

	log.Info("%s", msg)
}
