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

// PackageOf 외부 테스트 패키지에서 packageOf 를 검증하려고 노출한다.
var PackageOf = packageOf

// ResetCallSiteState 등록된 헬퍼 패키지, 호출 위치 캐시, 학습된 깊이를 모두 초기 상태로 되돌린다.
func ResetCallSiteState() {
	callSiteMu.Lock()
	defer callSiteMu.Unlock()

	helperPackages.Store(nil)
	learnedSpan.Store(0)
	resetCallSites()
}

// CachedCallSiteCount 캐시된 호출 위치 수를 반환한다.
func CachedCallSiteCount() int {
	cached := callSites.Load()
	if cached == nil {
		return 0
	}

	return len(*cached)
}

// LearnedSpan 학습된 언와인드 깊이를 반환한다.
func LearnedSpan() int {
	return int(learnedSpan.Load())
}
