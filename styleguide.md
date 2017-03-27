# 1 General
Of course our project must obey most rules of [Effective Go](https://golang.org/doc/effective_go.html) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

# 2 Message Format
Messages must be precise and concise, so there is no need to spell a completely right English sentence. We recommand using lowercase letters to spell words except the words are uppercase originally. So we can use text utils to batch processing them conveniently. And another tip: log an error or return it, don't do both.

## 2.1 Logger Format
If the context of logging is a function, the logger format must be:
`[message]`

e.g.
```go
logger.Debugf("[server already started]")`
```

If there are some important/special messages showed to log observers, you can use `: ` to emphasize them.
e.g
```go
logger.Debugf("[add configuration of plugin: %s]", plugin.Name)
```

We only support logger.Debugf/Infof/Warningf/Errorf because they can cover all functionality of Debug/Info/Warning/Error and Debugln/Infoln/Warningln/Errorln.
And the most important point is consistency, therefore we can use `grep logger.Debugf` to find all code about logging debug-level messages, same as others.

## 2.2 Error format
We recommend using `fmt.Errorf` instead of `errors.New`, the reason is same as above. `fmt.Errorf` can cover all functionality of `errors.New`, and we can use `grep fmt.Errorf` to find all user-defeined error message.

The error format would better be:
`act on xxx failed: failed reason`

e.g.
```go
fmt.Errorf("read file %s failed: %s", s.pluginFileFullPath, err)
```

If the error is not produced by a specified action, you could just give the reason which can also use `: ` to emphasize some values.

e.g.
```go
fmt.Errorf("duplicate configuration of plugin: %s", plugin.Name)
```
