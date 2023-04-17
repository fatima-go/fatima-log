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
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	TIME_YYYYMMDD = "2006-01-02"
	Hertz         = 100 // general linux CLK_TCK
)

func writeLogEvent(log LogEvent) {
	log.publish()
	if logPreference.streamMode == STREAM_MODE_STDOUT {
		fmt.Printf("%s", log.getMessage())
	} else {
		ensureLogFileExist()
		ensureTodayLog(log.getTime())
		writeLogEventToFile(log.getMessage())
	}
}

func ensureTodayLog(t time.Time) {
	if logPreference.currentLogFileTime.Year() != t.Year() ||
		logPreference.currentLogFileTime.Month() != t.Month() ||
		logPreference.currentLogFileTime.Day() != t.Day() {
		moveToBackupLog()
	}
}

func ensureLogFileExist() {
	if logPreference.logFileLoaded {
		return
	}

	var err error
	var stat os.FileInfo

	stat, err = os.Stat(logPreference.logFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			logPreference.logFilePtr, err = os.Create(logPreference.logFilePath)
			if err != nil {
				fmt.Printf("%s fail to create : %s", logPreference.logFilePath, err)
				logPreference.logFilePtr = nil
				return
			}
			logPreference.currentLogFileTime = time.Now()
		} else if stat.IsDir() {
			fmt.Printf("%s path exist as directory. fail to logging", logPreference.logFilePath)
			logPreference.logFilePtr = nil
		}
	} else {
		logPreference.logFilePtr, err = os.OpenFile(logPreference.logFilePath, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			fmt.Printf("fail to open : %s", err)
			logPreference.logFilePtr = nil
		}
		logPreference.currentLogFileTime = stat.ModTime()
	}

	logPreference.logFileLoaded = true
}

func moveToBackupLog() {
	var err error
	var stat os.FileInfo

	stat, err = os.Stat(logPreference.logFilePath)
	if err != nil {
		fmt.Printf("fail to stat log file : %s\n", err)
		logPreference.logFilePtr = nil
		return
	}

	// close current log file ptr
	if logPreference.logFilePtr != nil {
		logPreference.logFilePtr.Close()
		logPreference.logFilePtr = nil
	}

	// move current file to backup
	backupFilePath := fmt.Sprintf("%s%c%s.%s.log",
		logPreference.logFolder,
		filepath.Separator,
		logPreference.ProcessName, stat.ModTime().Format(TIME_YYYYMMDD))
	err = os.Rename(logPreference.logFilePath, backupFilePath)
	if err != nil {
		fmt.Printf("fail to rename [%s] -> [%s] : %s\n", logPreference.logFilePath, backupFilePath, err.Error())
	}

	go func() {
		removeOldLogFiles()
	}()

	// wait for file-io cache released : skip 1 tick
	time.Sleep(time.Millisecond * time.Duration(1000/Hertz))

	// open for new log file
	logPreference.logFilePtr, err = os.OpenFile(logPreference.logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Printf("fail to open for new log file : %s\n", err.Error())
		logPreference.logFilePtr = nil
		return
	}

	logPreference.currentLogFileTime = time.Now()
}

func writeLogEventToFile(s string) (n int, err error) {
	if logPreference.logFilePtr == nil {
		return 0, nil
	}
	return logPreference.logFilePtr.WriteString(s)
}

func removeOldLogFiles() {
	if logPreference.KeepingFileDays < 1 {
		return
	}

	// find files in log path
	files, err := ioutil.ReadDir(logPreference.logFolder)
	if err != nil {
		return
	}

	express := fmt.Sprintf("%s\\.[0-9]+-[0-9]+-[0-9]+\\.log", logPreference.ProcessName)
	var validLogFileId = regexp.MustCompile(express)
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "log") {
			continue
		}

		if !validLogFileId.MatchString(file.Name()) {
			continue
		}

		dotIndex := strings.Index(file.Name(), ".")
		if dotIndex < 1 {
			continue
		}

		lastDotIndex := strings.LastIndex(file.Name(), ".")
		if lastDotIndex <= dotIndex {
			continue
		}

		createdDateExpression := file.Name()[dotIndex+1 : lastDotIndex]
		createdDate, err := time.ParseInLocation(TIME_YYYYMMDD, createdDateExpression, time.Local)
		if err != nil {
			continue
		}

		diff := time.Duration(24*logPreference.KeepingFileDays) * time.Hour
		deadline := time.Now().Add(-diff)
		if createdDate.Before(deadline) {
			os.Remove(filepath.Join(logPreference.logFolder, file.Name()))
		}
	}
}
