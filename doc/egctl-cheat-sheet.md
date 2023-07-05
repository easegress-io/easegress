# egctl Cheat Sheet

## Creating resources
Easegress manifests are defined in YAML. The file extension .yaml and .yml be used. Use `egctl create`, or `egctl apply` to create resources. Use `egctl api-resources` to view all available resources and their supported actions. 

```
egctl apply -f ./pipeline-demo.yaml     # create resource object
egctl create -f ./httpserver-demo.yaml  # create resource object

egctl apply -f ./cdk-demo.yaml         # create resource customdatakind
egctl create -f custom-data-demo.yaml  # create resource customdata
```

## Viewing and finding resources 

```
egctl get all                         # list all resources
egctl get httpserver httpserver-demo  # find httpserver with name httpserver-demo

egctl get member                  # list all easegress nodes
egctl get member eg-default-name  # find easegress node with name eg-default-name

egctl get customdatakind          # list all custom data kind
egctl get customdata cdk-demo     # find custom data kind cdk-demo
 
egctl describe httpserver              # describe all httpserver
egctl describe pipeline pipeline-demo  # describe pipeline pipeline-demo
```

## Updating resources
```
egctl apply -f httpserver-demo-version2.yaml  # update object with new yaml file.
egctl apply -f cdk-demo2.yaml                 # udpate custom data kind with new yaml file.
```

## Deleting resources
```
egctl delete httpserver httpserver-demo   # delete httpserver httpserver-demo
egctl delete httpserver --all             # delete all httpserver
egctl delete customdatakind cdk-demo cdk-kind  # delete custom data kind cdk-demo and cdk-kind
```

## Other commands
```
egctl api-resources    # list all available resources 
egctl completion zsh   # generate completion script for zsh
egctl health           # check easegress health

egctl profile info                     # print profile file location.
egctl profile start cpu ./cpu-profile  # start profile cpu and put output in ./cpu-profile file
egctl profile stop                     # stop profile
```

## Config

By default, `egctl` looks for a file named `.egctlconfig` in the `$HOME` directory. Here's an example about `.egctlconfig`.

```yaml
kind: Config

# current used context.
current-context: context-default

# contexts contains user and cluster information, tell egctl use which user to access which cluster.
contexts:
  - context:
      cluster: cluster-default
      user: user-default
    name: context-default

# clusters contains cluster information.
# `server` is the host address egctl to access.
# `certificate-authority` or `certificate-authority-data` contains the root certificate authority that client uses when verifying server certificates.
clusters:
  - cluster:
      server: localhost:2381
      certificate-authority: "/tmp/certs/ca.crt"
	  certificate-authority-data: "xxxx" # base64
    name: cluster-default

# users contains user information.
# `username` and `password` is used as basic auth. 
# (`client-key`, `client-certificate`) or (`client-key-data`, `client-certificate-data`) contains client certificate.
users:
  - name: user-default
    user:
      username: user123
      password: password
      client-certificate: "/tmp/certs/client.crt"
      client-key: "/tmp/certs/client.key"
```

To use basic auth of `username` and `password`, add following part to easegress config yaml:

```yaml
name: easegress-1
cluster:
  listen-peer-urls:
  - http://localhost:2380
  ...
...
basic-auth:
  user123: password
  admin: admin
  username: password
```

To check client certificate, add following part to easegress config yaml:

```yaml
name: easegress-1
cluster:
  listen-peer-urls:
  - http://localhost:2380
  ...
...
# non empty client-ca-file will force server to check client certificate.
client-ca-file: "/tmp/certs/ca.crt"
```

```
egctl config current-context     # show current context used for egctl
egctl config get-contexts        # show all available contexts
egctl config use-context <name>  # update .egctlconfig file `current-context` to <name>
egctl config view                # view config file
```
