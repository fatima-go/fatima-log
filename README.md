# Golang logging library #

Package fatima-go/fatima-log implements a logging infrastructure for Go. 

```
# without ShowMethod
2017-04-19 18:09:07.616 INFO  [          t.j.e.http_server:63] Juno HttpServer Initialize()
2017-04-19 18:09:07.616 INFO  [          t.j.e.http_server:76] web guard listen : 10.0.1.2:9180
2017-04-19 18:09:07.616 INFO  [          t.j.w.web_service:67] using url seed : FCSTdnEC
2017-04-19 18:09:07.616 INFO  [          t.j.e.system_base:56] SystemBase Initialize()
2017-04-19 18:09:07.616 INFO  [         t.j.e.http_server:114] Juno HttpServer Bootup()
2017-04-19 18:09:07.616 INFO  [         t.j.e.http_server:127] called StartListening()
2017-04-19 18:09:07.616 INFO  [         t.j.e.http_server:136] start web guard listening...
2017-04-19 18:09:07.616 INFO  [         t.j.e.system_base:105] SystemBase Bootup()

# with ShowMethod
2017-04-19 18:05:37.979 INFO  [   t.j.e.http_server.Initialize():93] JupiterHttpServer Initialize()
2017-04-19 18:05:37.979 INFO  [  t.j.e.http_server.Initialize():106] web server listen : 0.0.0.0:9190
2017-04-19 18:05:37.980 DEBUG [.repo_file_user.loadXmlUserData():62] using xml : /IronForge/FATIMA_HOME/data/jupiter/user_data.xml
2017-04-19 18:05:37.980 INFO  [      t.j.e.http_server.Bootup():140] JupiterHttpServer Bootup()
2017-04-19 18:05:37.980 INFO  [j.e.http_server.StartListening():152] called JupiterHttpServer StartListening()
2017-04-19 18:05:37.980 INFO  [j.e.http_server.StartListening():161] start web server listening...
```

# Install #
```
go get github.com/fatima-go/fatima-log
```


# Example #

```
#!go

package main

import (
	"github.com/fatima-go/fatima-log"
	"time"
	"errors"
)

func main() {
	// need folder path for logging file
	logpath := "/somewhere/logs"

	// you have to create logging preference first
	// if you use empty logpath(""), then log will be written to stdout
	pref := log.NewPreference(logpath)

	// set some properties for logging
	pref.ShowMethod = true
	pref.DefaultLogLevel = log.LOG_DEBUG

	// initialize logging
	log.Initialize(pref)

	// do log
	log.Info("%s", time.Now())
	log.Debug("you can write debug messages...")
	log.Warn("logpath=%s, logpath.len=%d", logpath, len(logpath))
	createError()
}

func createError()  {
	log.Error("error catched", errors.New("sample error"))
}
```

```
# result log message of sample code

2017-04-19 18:45:01.050 INFO  [          q.queryman.main():37] 2017-04-19 18:45:01.050525314 +0900 KST
2017-04-19 18:45:01.050 DEBUG [          q.queryman.main():38] you can write debug messages...
2017-04-19 18:45:01.050 WARN  [          q.queryman.main():39] logpath=/somewhere/logs, logpath.len=29
2017-04-19 18:45:01.050 ERROR [   q.queryman.createError():47] error catched
	(*errors.errorString) :: sample error
	TRACE <<<
	[main(), queryman.queryman.go:40]
	[main(), runtime.proc.go:185]
	[goexit(), runtime.asm_amd64.s:2197]

```

# Logging Properties #

You can set logging preference. below is preference properties

name     | type   | default | remark
---------:| :----- | :----- | :-----
ShowMethod  |  bool | false | whether show method name or not
KeepingFileDays | uint16 | 90 | max days for keeping log files
SourcePrintSize | uint8 | 30 | source expression length
LogfileSizeLimitMB | uint16 | 0 | max log file size in MB (not yet supported)
MaxErrorTraceLevel | uint8 | 10 | max trace level for error
ProcessName | string | program name | running program(process) name
DefaultLogLevel | LogLevel | TRACE | default logging level
DeliveryMode | LogDeliveryMode | DELIVERY_MODE_SYNC | sync or async

## Sentry Integration ##
func SetSentryDsn(dsn string, tags map[string]string)
- dsn : sentry dsn url
- tags : sentry tags...

func SetSentryFlushSecond(second int)
- default 2 seconds

func SetSentryLogLevel(logLevel string)
- default "error"

```
// example code

m := make(map[string]string)
m["environment"] = "prod"
m["serverName"] = "event01"
m["process"] = "myprocess"
log.SetSentryDsn(dsn, m)
log.SetSentryLogLevel("warn")
log.SetSentryFlushSecond(1)
```

## DeliveryMode ##

* DELIVERY_MODE_SYNC
	* write message to file while logging event occured
* DELIVERY_MODE_ASYNC
	* write message using go func(). it could improve program execution because logging will be detached from main.
	* if you use ASYNC mode, you have to call `log.Close()` when your program exit. if not, you may lost some last logging message

# log folder sample #
```
OSX:juno fatima-go$ ls -ltr
total 4200
-rw-r--r--  1 fatima-go  staff     7654  3  8 16:15 juno.2017-03-08.log
-rw-------  1 fatima-go  staff    10880  3 11 22:57 juno.2017-03-11.log
-rw-------  1 fatima-go  staff    75197  3 12 23:02 juno.2017-03-12.log
-rw-------  1 fatima-go  staff    18801  3 13 09:49 juno.2017-03-13.log
-rw-------  1 fatima-go  staff    10189  3 14 18:29 juno.2017-03-14.log
-rw-------  1 fatima-go  staff    27583  3 15 13:37 juno.2017-03-15.log
-rw-------  1 fatima-go  staff    57586  3 16 19:13 juno.2017-03-16.log
-rw-------  1 fatima-go  staff   112852  3 17 23:59 juno.2017-03-17.log
-rw-------  1 fatima-go  staff   322963  3 18 16:35 juno.2017-03-18.log
-rw-------  1 fatima-go  staff  1208322  3 24 23:30 juno.2017-03-24.log
-rw-------  1 fatima-go  staff    39266  3 25 23:52 juno.2017-03-25.log
-rw-------  1 fatima-go  staff   148527  3 26 19:16 juno.2017-03-26.log
-rw-------  1 fatima-go  staff    45154  4 14 23:26 juno.2017-04-14.log
-rw-------  1 fatima-go  staff    32544  4 17 18:00 juno.2017-04-17.log
-rw-------  1 fatima-go  staff     2334  4 19 18:09 juno.log
OSX:juno fatima-go$
```