# Log

- [Default Configuration](#default-configuration)
- [Basic Log Spec](#basic-log-spec)
- [Custom Log Configuration](#custom-log-configuration)

Now, easgress support rotating log files like `logrotate`.
If startup `easegress-server` with `--log-dir={log-path}` param specified and without `-log-config`, it will use the default configuration to manage the log files.

## Default Configuration
| Name | Type | Description | Required  |
|------- |--------|-------------|-----|
| stdLog | [LogSpec](#BasicLogSpec) | The log configruation for `logger.Debugf`,`logger.Infof`,`logger.LazyDebug`,`logger.Warnf` ,`logger.Errorf` | no |
| accessLog| [LogSpec](#BasicLogSpec) | The log configruation for `logger.HTTPAccess`,`logger.LazyHTTPAccess` | no |
| dumpLog | [LogSpec](#BasicLogSpec) | The log configruation for dumping some snapshot | no |
| adminAPLog | [LogSpec](#BasicLogSpec) | The log configuration for `logger.APIAccess` | no |
| etcdServerLog | [LogSpec](#BasicLogSpec) | The log configuration for embed etcd-server | no |
| etcdClientLog | [LogSpec](#BasicLogSpec) | The log configuration for embed etcd-client and EtcdServiceRegistry | no |
| oTel | [LogSpec](#BasicLogSpec) | The log configuration for opentelemetry otel | no |

```
		StdLog: &Spec{
			FileName:   "stdout.log",
			MaxSize:    128,
			MaxAge:     30,
			MaxBackups: 60,
			LocalTime:  true,
			Compress:   true,
		},
		Access: &Spec{
			FileName:   "filter_http_access.log",
			MaxSize:    128,
			MaxAge:     30,
			MaxBackups: 60,
			LocalTime:  true,
			Compress:   true,
		},
		Dump: &Spec{
			FileName:   "filter_http_dump.log",
			MaxSize:    128,
			MaxAge:     30,
			MaxBackups: 60,
			LocalTime:  true,
			Compress:   true,
		},
		AdminAPI: &Spec{
			FileName:   "admin-api.log",
			MaxSize:    128,
			MaxAge:     30,
			MaxBackups: 60,
			LocalTime:  true,
			Compress:   true,
		},
		EtcdServer: &Spec{
			FileName:   "etcd-server.log",
			MaxSize:    128,
			MaxAge:     30,
			MaxBackups: 60,
			LocalTime:  true,
			Compress:   true,
		},
		EtcdClient: &Spec{
			FileName:   "etcd-client.log",
			MaxSize:    128,
			MaxAge:     30,
			MaxBackups: 60,
			LocalTime:  true,
			Compress:   true,
		},
		OTel: &Spec{
			FileName:   "otel.log",
			MaxSize:    128,
			MaxAge:     30,
			MaxBackups: 60,
			LocalTime:  true,
			Compress:   true,
		},
```

## Basic Log Spec
| Name | Type | Description | Required |
|-----|-----|----|----|
| fileName | string | The name of Log File | yes |
| maxSize | int | The maximum size in megabytes of the log file before it gets rotated. It defaults to 100 megabytes | no |
| maxAge | int | The maximum number of days to retain old log files based on the timestamp encoded in their filename.  Note that a day is defined as 24 hours and may not exactly correspond to calendar days due to daylight savings, leap seconds, etc. The default is not to remove old log files based on age | no |
| maxBackups | int | The maximum number of old log files to retain, and backup log files will be retained in the same directory. The default is to retain all old log files (though MaxAge may still cause them to get deleted.) | no |
| localTime | bool | It determines if the time used for formatting the timestamps in backup files is the computer's local time. The default is to use UTC time | no |
| compress | bool | It determines if the rotated log files should be compressed using gzip. The default is not to perform compression | no |
| perm | string | It defines a file's access permissions. default '0o644' |no|

## Custom Log Configuration
Startup `easegress-server` with `--log-dir={log-path}` and `--log-config={log-config-file-path}` param specified.

Below is an example of the configuration file content
```
stdLog:
   fileName: masa-bigress.log
   maxSize: 256
   maxAge: 30
   maxBackups: 60
   localtime: true
   compress: true
   perm: "0600"
accessLog:
   fileName: access.log
   maxSize: 256
   maxAge: 30
   maxBackups: 60
   localtime: true
   compress: true
   perm: "0o640"
dumpLog:
   fileName: dump.log
   maxSize: 256
   maxAge: 30
   maxBackups: 60
   localtime: true
   compress: true
adminAPLog:
   fileName: admin-api.log
   maxSize: 256
   maxAge: 30
   maxBackups: 60
   localtime: true
   compress: true
etcdServerLog:
   fileName: etcd-server.log
   maxSize: 256
   maxAge: 30
   maxBackups: 60
   localtime: true
   compress: true
etcdClientLog:
   fileName: etcd-client.log
   maxSize: 256
   maxAge: 30
   maxBackups: 60
   localtime: true
   compress: true
oTel:
   fileName: otel.log
   maxSize: 256
   maxAge: 30
   maxBackups: 60
   localtime: true
   compress: true
```