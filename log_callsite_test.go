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

package log_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	log "github.com/fatima-go/fatima-log"
	"github.com/fatima-go/fatima-log/internal/calltest"
)

const (
	testProcName        = "callsite"
	testSourcePrintSize = 120
)

// logFilePath TestMain 이 준비한 로그 파일 경로다.
var logFilePath string

// TestMain 로거는 프로세스에서 한 번만 초기화되므로, 임시 폴더에 동기 모드로 한 번 초기화해 모든 테스트가 같은 파일을 읽는다.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "fatima-log-test")
	if err != nil {
		fmt.Printf("fail to create temp dir : %s\n", err)
		os.Exit(1)
	}

	pref := log.NewPreferenceWithProcName(dir, testProcName)
	pref.ShowMethod = true
	// 소스 표기가 잘리지 않아야 함수명과 라인을 그대로 비교할 수 있다.
	pref.SourcePrintSize = testSourcePrintSize
	log.Initialize(pref)
	logFilePath = filepath.Join(dir, testProcName+".log")

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// nextLine 이 함수를 호출한 줄의 바로 다음 줄 번호를 반환한다.
func nextLine() int {
	_, _, line, _ := runtime.Caller(1)
	return line + 1
}

// resetState 테스트 사이에 헬퍼 등록과 캐시, 테스트가 바꾼 설정이 새지 않게 한다.
func resetState(t *testing.T) {
	t.Helper()
	log.ResetCallSiteState()
	t.Cleanup(func() {
		log.SetShowMethod(true)
		log.SetSourcePrintSize(testSourcePrintSize)
		log.ResetCallSiteState()
	})
}

// eventOf 로그 파일에서 marker 가 들어간 마지막 줄과 그 뒤에 이어지는 줄들을 반환한다.
func eventOf(t *testing.T, marker string) []string {
	t.Helper()
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("fail to read log file : %s", err)
	}

	// 같은 프로세스에서 테스트를 반복 실행(-count)해도 이번 실행의 줄을 보도록 마지막으로 나온 줄을 찾는다.
	lines := strings.Split(string(content), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], marker) {
			return lines[i:]
		}
	}

	t.Fatalf("log line not found : marker=%s", marker)
	return nil
}

// sourceOf 로그 줄의 "[...]" 안 소스 표기를 반환한다.
func sourceOf(t *testing.T, line string) string {
	t.Helper()
	begin := strings.IndexByte(line, '[')
	end := strings.IndexByte(line, ']')
	if begin < 0 || end < begin {
		t.Fatalf("source not found : line=%s", line)
	}

	return line[begin+1 : end]
}

// assertSource 소스 표기가 "<파일>.<함수>():<라인>" 으로 끝나는지 확인한다.
func assertSource(t *testing.T, marker, file, function string, line int) {
	t.Helper()
	got := sourceOf(t, eventOf(t, marker)[0])
	want := fmt.Sprintf("%s.%s():%d", file, function, line)
	if !strings.HasSuffix(got, want) {
		t.Errorf("unexpected source : marker=%s, got=%q, want suffix=%q", marker, got, want)
	}
}

func TestDirectCallKeepsCallerLocation(t *testing.T) {
	resetState(t)

	want := nextLine()
	log.Info("%s", "direct-call")

	assertSource(t, "direct-call", "log_callsite_test", "TestDirectCallKeepsCallerLocation", want)
	if got := log.LearnedSpan(); got != 1 {
		t.Errorf("unexpected learned span : got=%d, want=1", got)
	}
}

func TestHelperPackageBeforeRegister(t *testing.T) {
	resetState(t)

	calltest.Trace{}.Info("before-register")

	got := sourceOf(t, eventOf(t, "before-register")[0])
	if !strings.Contains(got, "calltest.Info()") {
		t.Errorf("unregistered helper should be printed as source : got=%q", got)
	}
}

func TestHelperPackageResolvesCaller(t *testing.T) {
	resetState(t)
	calltest.Register()

	trace := calltest.Trace{}
	tests := []struct {
		name string
		emit func(msg string)
	}{
		{name: "depth1", emit: trace.Info},
		{name: "depth2", emit: trace.Finish},
		{name: "depth3", emit: trace.Done},
		{name: "pointer-receiver", emit: (&trace).Warn},
		{name: "closure", emit: calltest.Closure},
		{name: "generic", emit: calltest.Generic[string]},
	}

	for _, tc := range tests {
		marker := "resolved-" + tc.name
		// 같은 호출 지점을 두 번 지나가게 해 캐시 적중 경로도 같은 결과를 내는지 본다.
		for round := 0; round < 2; round++ {
			want := nextLine()
			tc.emit(fmt.Sprintf("%s-%d", marker, round))

			assertSource(t, fmt.Sprintf("%s-%d", marker, round), "log_callsite_test", "TestHelperPackageResolvesCaller", want)
		}
	}

	// 가장 깊은 경로(Done → Finish → Info)의 헬퍼 3단 + 호출부 1 을 학습해야 한다.
	if got := log.LearnedSpan(); got != 4 {
		t.Errorf("unexpected learned span : got=%d, want=4", got)
	}
}

func TestCallSiteCacheIsReused(t *testing.T) {
	resetState(t)
	calltest.Register()

	trace := calltest.Trace{}
	// 인라인되면 호출 지점마다 헬퍼 pc 가 따로 생기므로, 반드시 같은 호출 지점을 반복해서 지나가게 한다.
	var cached int
	for i := 0; i < 50; i++ {
		trace.Done(fmt.Sprintf("cache-reuse-%d", i))
		if i == 0 {
			cached = log.CachedCallSiteCount()
		}
	}

	if cached == 0 {
		t.Fatalf("call site should be cached")
	}
	if got := log.CachedCallSiteCount(); got != cached {
		t.Errorf("cache should not grow for the same call site : got=%d, want=%d", got, cached)
	}
}

func TestRegisterAfterCachedResetsCache(t *testing.T) {
	resetState(t)

	trace := calltest.Trace{}
	var want int
	emit := func(msg string) {
		want = nextLine()
		trace.Info(msg)
	}

	emit("register-late-before")
	if got := sourceOf(t, eventOf(t, "register-late-before")[0]); !strings.Contains(got, "calltest.Info()") {
		t.Fatalf("unexpected source before register : got=%q", got)
	}

	// 같은 pc 가 "헬퍼 아님" 으로 캐시된 뒤에 등록해도 바로 반영돼야 한다.
	calltest.Register()
	emit("register-late-after")

	assertSource(t, "register-late-after", "log_callsite_test", "func1", want)
}

func TestSettersResetCachedSource(t *testing.T) {
	resetState(t)
	calltest.Register()

	trace := calltest.Trace{}
	var want int
	emit := func(msg string) {
		want = nextLine()
		trace.Done(msg)
	}

	emit("setter-default")
	assertSource(t, "setter-default", "log_callsite_test", "func1", want)

	log.SetShowMethod(false)
	emit("setter-no-method")
	got := sourceOf(t, eventOf(t, "setter-no-method")[0])
	if !strings.HasSuffix(got, fmt.Sprintf("log_callsite_test:%d", want)) {
		t.Errorf("show method off should drop method name : got=%q", got)
	}

	log.SetSourcePrintSize(12)
	emit("setter-print-size")
	got = sourceOf(t, eventOf(t, "setter-print-size")[0])
	if len(got) != 12 || !strings.HasSuffix(got, fmt.Sprintf(":%d", want)) {
		t.Errorf("print size should be applied to cached call site : got=%q", got)
	}
}

// failFrom 오류 로그의 호출 위치(이 함수)와 트레이스 첫 줄(이 함수를 부른 쪽)을 구분하려고 둔다.
//
//go:noinline
func failFrom(emit func(msg string), msg string) int {
	want := nextLine()
	emit(msg)
	return want
}

func TestErrorTraceStartsAboveCallSite(t *testing.T) {
	trace := calltest.Trace{}
	tests := []struct {
		name     string
		register bool
		emit     func(msg string)
	}{
		{name: "direct", emit: func(msg string) { log.Error(msg, errors.New("boom")) }},
		{name: "helper", register: true, emit: func(msg string) { trace.Fail(msg, errors.New("boom")) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetState(t)
			if tc.register {
				calltest.Register()
			}

			marker := "error-trace-" + tc.name
			// emit 클로저가 실제 로그 호출부이므로, 트레이스는 그 위 프레임인 failFrom 부터 시작해야 한다.
			callLine := failFrom(tc.emit, marker)

			event := eventOf(t, marker)
			if len(event) < 4 {
				t.Fatalf("error event should have trace lines : %v", event)
			}
			if !strings.Contains(event[1], "boom") || !strings.Contains(event[2], "TRACE <<<") {
				t.Errorf("unexpected error header : %q / %q", event[1], event[2])
			}

			if !strings.Contains(event[3], "[failFrom(), ") || !strings.HasSuffix(event[3], fmt.Sprintf("log_callsite_test.go:%d]", callLine)) {
				t.Errorf("unexpected first trace point : got=%q, want failFrom at line %d", event[3], callLine)
			}
		})
	}
}

// printfLibrary 주입받은 로거를 한 단계 감싸 호출하는 외부 라이브러리를 흉내낸다.
//
//go:noinline
func printfLibrary(logger interface{ Printf(string, ...interface{}) }, msg string) {
	logger.Printf("%s", msg)
}

func TestCustomLoggerKeepsSkipLevel(t *testing.T) {
	resetState(t)

	logger := log.NewCustomLogger("info")
	want := nextLine()
	printfLibrary(logger, "custom-logger")

	assertSource(t, "custom-logger", "log_callsite_test", "TestCustomLoggerKeepsSkipLevel", want)
}

func TestCustomLoggerErrorTraceStartsAboveCallSite(t *testing.T) {
	resetState(t)

	logger := log.NewCustomLogger("info")
	library := func(msg string) { logger.Printf(msg, errors.New("boom")) }

	callLine := failFrom(library, "custom-logger-error")

	// Printf 는 자신을 부른 라이브러리의 호출자(failFrom)를 호출 위치로 보므로, 트레이스는 그 위인 이 테스트부터다.
	assertSource(t, "custom-logger-error", "log_callsite_test", "failFrom", callLine)
	event := eventOf(t, "custom-logger-error")
	if len(event) < 4 || !strings.Contains(event[3], "[TestCustomLoggerErrorTraceStartsAboveCallSite(), ") {
		t.Errorf("unexpected first trace point : %v", event)
	}
}

// recurse 스택을 depth 만큼 쌓은 뒤 emit 을 호출한다.
//
//go:noinline
func recurse(depth int, emit func()) {
	if depth > 0 {
		recurse(depth-1, emit)
		return
	}

	emit()
}

func TestErrorTracePointCount(t *testing.T) {
	resetState(t)
	calltest.Register()

	trace := calltest.Trace{}
	recurse(20, func() { trace.Fail("trace-count", errors.New("boom")) })

	points := 0
	for _, line := range eventOf(t, "trace-count")[1:] {
		if strings.HasPrefix(line, "\t[") {
			points++
		}
	}

	// 기본 MaxErrorTraceLevel(10) 에서 3 을 뺀 개수다. v1.0.2 와 같아야 한다.
	if points != log.DEFAULT_ERROR_TRACE_LEVEL-3 {
		t.Errorf("unexpected trace point count : got=%d, want=%d", points, log.DEFAULT_ERROR_TRACE_LEVEL-3)
	}
}

func TestGoroutineStartedInHelperFallsBackToHelper(t *testing.T) {
	resetState(t)
	calltest.Register()

	calltest.Async("async-in-helper")

	// 스택에 헬퍼 밖 호출부가 없다. runtime.goexit 가 아니라 가장 바깥 헬퍼 프레임이 찍혀야 한다.
	got := sourceOf(t, eventOf(t, "async-in-helper")[0])
	if !strings.Contains(got, "calltest.") || strings.Contains(got, "goexit") {
		t.Errorf("unexpected source : got=%q", got)
	}
}

func TestTooDeepHelperChainFallsBackToHelper(t *testing.T) {
	resetState(t)
	calltest.Register()

	calltest.Deep(20, "too-deep-helper")

	got := sourceOf(t, eventOf(t, "too-deep-helper")[0])
	if !strings.Contains(got, "calltest.Deep()") {
		t.Errorf("unexpected source : got=%q", got)
	}
}

// panicWithDeferredHelper 헬퍼를 defer 로 걸어 둔 채 panic 을 낸다. 반환값은 panic 을 낸 줄이다.
//
//go:noinline
func panicWithDeferredHelper(trace calltest.Trace, msg string) (line int) {
	defer func() { _ = recover() }()
	defer trace.Done(msg)

	line = nextLine()
	panic("boom")
}

func TestDeferredHelperDuringPanicResolvesCaller(t *testing.T) {
	resetState(t)
	calltest.Register()

	want := panicWithDeferredHelper(calltest.Trace{}, "deferred-in-panic")

	// 헬퍼와 호출부 사이에 runtime.gopanic 이 끼어도 panic 을 낸 함수가 찍혀야 한다.
	assertSource(t, "deferred-in-panic", "log_callsite_test", "panicWithDeferredHelper", want)
}

func TestConcurrentLoggingWithSetterChanges(t *testing.T) {
	resetState(t)
	calltest.Register()

	trace := calltest.Trace{}
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				trace.Done(fmt.Sprintf("concurrent-%d-%d", worker, i))
			}
		}(worker)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			log.SetShowMethod(i%2 == 0)
		}
	}()
	wg.Wait()

	log.SetShowMethod(true)
	want := nextLine()
	trace.Done("concurrent-final")

	assertSource(t, "concurrent-final", "log_callsite_test", "TestConcurrentLoggingWithSetterChanges", want)
}

func TestPackageOf(t *testing.T) {
	tests := []struct {
		name     string
		function string
		want     string
	}{
		{name: "function", function: "github.com/fatima-go/fatima-log.Info", want: "github.com/fatima-go/fatima-log"},
		{name: "value method", function: "github.com/acme/app/trace.Trace.Info", want: "github.com/acme/app/trace"},
		{name: "pointer method", function: "github.com/acme/app/trace.(*Trace).Warn", want: "github.com/acme/app/trace"},
		{name: "closure", function: "github.com/acme/app/trace.Closure.func1", want: "github.com/acme/app/trace"},
		{name: "generic", function: "github.com/acme/app/trace.Generic[...]", want: "github.com/acme/app/trace"},
		{name: "escaped dot in path", function: "gopkg.in/yaml%2ev3.Unmarshal", want: "gopkg.in/yaml%2ev3"},
		{name: "no slash", function: "main.main", want: "main"},
		{name: "no dot", function: "goexit", want: "goexit"},
		{name: "empty", function: "", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := log.PackageOf(tc.function); got != tc.want {
				t.Errorf("unexpected package : got=%q, want=%q", got, tc.want)
			}
		})
	}
}
