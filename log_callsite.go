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

package log

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	// maxUnwindFrames 호출 위치를 찾으려고 거슬러 올라가는 최대 프레임 수다.
	maxUnwindFrames = 16
	// maxCachedCallSites 호출 위치 캐시 상한이다. pc 는 바이너리의 로그 호출 지점 수로 제한되므로 보통 닿지 않는 방어값이다.
	maxCachedCallSites = 8192
)

// callSite pc 하나에 대응하는 호출 위치다. pc 가 같으면 함수·파일·라인이 항상 같으므로 한 번 만들어 바꾸지 않는다.
type callSite struct {
	helper bool
	// runtime Go 런타임(goexit, main, gopanic 등)의 프레임인지 여부다.
	runtime  bool
	file     string
	funcName string
	line     int
	// source 로그 줄의 "[...]" 안에 들어가는 완성된 소스 표기다. ShowMethod, SourcePrintSize 에 의존한다.
	source string
}

var (
	// callSiteMu 헬퍼 등록, 캐시 쓰기, 소스 표기에 영향을 주는 설정 변경을 직렬화한다. 캐시 적중 경로는 잡지 않는다.
	callSiteMu sync.Mutex
	// helperPackages 로깅 헬퍼로 등록된 패키지 경로 집합이다(copy-on-write).
	helperPackages atomic.Pointer[map[string]struct{}]
	// callSites pc 별 호출 위치 캐시다(copy-on-write). 읽기는 락과 할당 없이 이뤄진다.
	callSites atomic.Pointer[map[uintptr]*callSite]
	// learnedSpan 지금까지 관측한 "헬퍼 프레임 수 + 1" 의 최댓값이다. 필요한 만큼만 스택을 풀기 위해 쓴다.
	learnedSpan atomic.Int32
)

// RegisterHelperPackage 이 함수를 호출한 패키지를 로깅 헬퍼로 등록한다.
// 로그의 소스 위치를 찾을 때 등록된 패키지의 프레임은 건너뛰므로, 로깅을 별도 패키지로 감싸도 실제 호출부가 찍힌다.
// 헬퍼 패키지의 init() 에서 한 번 호출한다. 패키지 단위로 건너뛰므로 헬퍼 패키지는 로깅 전용으로 둔다.
func RegisterHelperPackage() {
	var pcs [1]uintptr
	// runtime.Callers 기준으로 Callers(0), RegisterHelperPackage(1) 다음인 2 가 호출한 쪽이다.
	if runtime.Callers(2, pcs[:]) == 0 {
		return
	}

	frame, _ := runtime.CallersFrames(pcs[:]).Next()
	registerHelperPackage(packageOf(frame.Function))
}

// registerHelperPackage 패키지 경로를 헬퍼로 등록한다.
// 이미 캐시된 호출 위치의 헬퍼 여부가 달라질 수 있으므로 캐시도 함께 비운다.
func registerHelperPackage(pkg string) {
	if len(pkg) == 0 {
		return
	}

	callSiteMu.Lock()
	defer callSiteMu.Unlock()

	next := map[string]struct{}{pkg: {}}
	if current := helperPackages.Load(); current != nil {
		if _, exist := (*current)[pkg]; exist {
			return
		}
		for k := range *current {
			next[k] = struct{}{}
		}
	}

	helperPackages.Store(&next)
	resetCallSites()
}

// resetCallSites 호출 위치 캐시를 비운다. callSiteMu 를 잡은 상태에서 호출한다.
// 학습된 깊이는 설정과 무관하므로 유지한다.
func resetCallSites() {
	callSites.Store(nil)
}

// resolveCallSite 헬퍼 패키지가 아닌 첫 프레임의 호출 위치와, 그 프레임까지 건너뛴 프레임 수를 반환한다.
// skip 은 runtime.Callers 기준이며 이 함수 자신이 1 이다.
//
// 헬퍼 프레임 위에 있는 런타임 프레임도 건너뛴다. 헬퍼를 defer 로 걸어 둔 채 panic 이 나면 그 사이에 runtime.gopanic 이 끼고,
// 헬퍼 안에서 시작한 고루틴은 헬퍼 바로 위가 runtime.goexit 이기 때문이다.
// 끝까지 호출부를 찾지 못하면 가장 바깥 헬퍼 프레임으로 대신한다. 헬퍼를 거치지 않은 호출은 첫 프레임을 그대로 쓴다.
func resolveCallSite(skip int) (*callSite, int) {
	var pcs [maxUnwindFrames]uintptr

	span := int(learnedSpan.Load())
	if span < 1 {
		span = 1
	}

	for {
		var outermostHelper *callSite
		outermostHelperIndex := 0

		n := runtime.Callers(skip, pcs[:span])
		for i := 0; i < n; i++ {
			site := lookupCallSite(pcs[i])
			if site.helper {
				outermostHelper, outermostHelperIndex = site, i
				continue
			}

			if site.runtime && outermostHelper != nil {
				continue
			}

			rememberSpan(i + 1)
			return site, i
		}

		if n == span && span < maxUnwindFrames {
			span = maxUnwindFrames
			continue
		}

		if outermostHelper == nil {
			// 스택을 얻지 못한 경우다. 빈 프레임으로 해석되는 pc 0 을 대신 쓴다.
			return lookupCallSite(0), 0
		}

		return outermostHelper, outermostHelperIndex
	}
}

// rememberSpan 관측한 깊이가 더 크면 학습된 깊이를 올린다.
func rememberSpan(span int) {
	for {
		current := learnedSpan.Load()
		if int32(span) <= current || learnedSpan.CompareAndSwap(current, int32(span)) {
			return
		}
	}
}

// lookupCallSite pc 의 호출 위치를 캐시에서 찾고, 없으면 해석해 캐시에 넣는다.
func lookupCallSite(pc uintptr) *callSite {
	if cached := callSites.Load(); cached != nil {
		if site, ok := (*cached)[pc]; ok {
			return site
		}
	}

	return learnCallSite(pc)
}

// learnCallSite pc 를 해석해 캐시에 넣는다.
// 설정 읽기부터 저장까지 락 안에서 수행해, 설정 변경(캐시 리셋) 직후에 옛 설정으로 만든 표기가 저장되는 일을 막는다.
func learnCallSite(pc uintptr) *callSite {
	// runtime.Callers 는 인라인된 프레임마다 pc 를 따로 돌려주므로 pc 하나는 논리 프레임 하나에 대응한다.
	frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()

	callSiteMu.Lock()
	defer callSiteMu.Unlock()

	cached := callSites.Load()
	if cached != nil {
		if site, ok := (*cached)[pc]; ok {
			return site
		}
	}

	site := newCallSite(frame)
	if cached != nil && len(*cached) >= maxCachedCallSites {
		return site
	}

	next := make(map[uintptr]*callSite, 1)
	if cached != nil {
		next = make(map[uintptr]*callSite, len(*cached)+1)
		for k, v := range *cached {
			next[k] = v
		}
	}
	next[pc] = site
	callSites.Store(&next)

	return site
}

// newCallSite 프레임 하나로 호출 위치를 만든다. 설정을 읽으므로 callSiteMu 를 잡은 상태에서 호출한다.
func newCallSite(frame runtime.Frame) *callSite {
	site := &callSite{
		file:     frame.File,
		funcName: functionNameOf(frame.Function),
		line:     frame.Line,
	}

	pkg := packageOf(frame.Function)
	site.runtime = pkg == "runtime"
	if helpers := helperPackages.Load(); helpers != nil {
		_, site.helper = (*helpers)[pkg]
	}

	site.source = buildSourceDescription(site, logPreference.ShowMethod, logPreference.SourcePrintSize)
	return site
}

// packageOf 함수 전체 이름에서 패키지 경로를 떼어낸다.
// 마지막 '/' 뒤의 첫 '.' 앞까지가 패키지다. 경로에 있는 '.' 은 심볼에서 "%2e" 로 바뀌므로 메서드·클로저·제네릭 모두 같은 규칙으로 처리된다.
func packageOf(function string) string {
	slash := strings.LastIndexByte(function, '/')
	dot := strings.IndexByte(function[slash+1:], '.')
	if dot < 0 {
		return function
	}

	return function[:slash+1+dot]
}

// functionNameOf 함수 전체 이름에서 마지막 '.' 뒤의 이름만 남긴다.
func functionNameOf(function string) string {
	found := strings.LastIndexByte(function, '.')
	if found < 0 {
		return function
	}

	return function[found+1:]
}

// buildSourceDescription 호출 위치의 소스 표기를 만든다.
// 경로는 "/src/"(없으면 "/mod/") 뒤부터 쓰고, 디렉토리는 첫 글자만 남기며, printSize 에 맞춰 앞을 자르거나 공백으로 채운다.
func buildSourceDescription(site *callSite, showMethod bool, printSize uint8) string {
	location := site.file
	if found := strings.LastIndex(site.file, "/src/"); found > 0 {
		location = site.file[found+5:]
	} else if found = strings.LastIndex(site.file, "/mod/"); found > 0 {
		location = site.file[found+5:]
	}

	var buffer bytes.Buffer
	tokens := strings.Split(location, "/")
	for i, s := range tokens {
		if len(s) == 0 {
			continue
		}

		if i < len(tokens)-1 {
			buffer.WriteByte(s[0])
			buffer.WriteByte('.')
		} else {
			buffer.WriteString(strings.TrimSuffix(s, ".go"))
		}
	}

	var message string
	if showMethod {
		message = fmt.Sprintf("%s.%s():%d", buffer.String(), site.funcName, site.line)
	} else {
		message = fmt.Sprintf("%s:%d", buffer.String(), site.line)
	}

	startIndex := len(message) - int(printSize)
	if startIndex >= 0 {
		return message[startIndex:]
	}

	return strings.Repeat(" ", -startIndex) + message
}
