cotest.sh will random select remote host and send random select command to remote host by SSH execute. The Script will infinite loop until Ctrl-C interrupted.
# 1.Set remote host array
- Make sure the remote host is configured SSH login without password.
- Set host infomation in `co_host` array. One row for one host information.
eg.
```
co_host=(
"ubuntu@192.168.50.101"
"ubuntu@192.168.50.102"
...
)
```
# 2.Set command array
- Set command that need execute in `co_command` array. One row for mulitple commands are executed at one time.
- If operation is complicated, maybe using a script in remote host and set a command call this script.
eg.
```
co_command=(
"PID=\`cat ~/easegress/easegress.pid\`;kill -USR2 \$PID"
)
```
# 3.Set sleep interval
- Set `minintv`, this is base interval
- Set `randintv`, this is upper limit of random interval, the real interval is `minintv + random(0,randintv)`
# 4.Test scenarios
- Send USR2 signal to event graceful update
```
  PID=\`cat ~/easegress/easegress.pid\`;kill -USR2 \$PID
```
