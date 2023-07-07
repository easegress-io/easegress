# egctl Cheat Sheet

## Creating resources

Easegress manifests are defined using YAML. They can be identified by the file extensions `.yaml` or `.yml`. You can create resources using either the `egctl create` or `egctl apply` commands. To view all available resources along with their supported actions, use the `egctl api-resources` command.

```
cat globalfilter.yaml | egctl create -f -   # create GlobalFilter resource from stdin
cat httpserver-new.yaml | egctl apply -f -  # create HTTPServer resource from stdin


egctl apply -f ./pipeline-demo.yaml      # create Pipeline resource
egctl create -f ./httpserver-demo.yaml   # create HTTPServer resource

egctl apply -f ./cdk-demo.yaml           # create CustomDataKind resource
egctl create -f ./custom-data-demo.yaml  # create CustomData resource
```

## Viewing and finding resources 

```
egctl get all                          # view all resources
egctl get httpserver httpserver-demo   # find HTTPServer resources with name "httpserver-demo"

egctl get member                       # view all easegress nodes
egctl get member eg-default-name       # find easegress node with name "eg-default-name"

egctl get customdatakind               # view all CustomDataKind resources
egctl get customdata cdk-demo          # find CustomDataKind resource with name "cdk-demo"
 
egctl describe httpserver              # describe all HTTPServer resource 
egctl describe pipeline pipeline-demo  # describe Pipeline resource with name "pipeline-demo"
```

## Updating resources
```
egctl apply -f httpserver-demo-version2.yaml  # update HTTPServer resource
egctl apply -f cdk-demo2.yaml                 # udpate CustomDataKind resource
```

## Deleting resources
```
egctl delete httpserver httpserver-demo        # delete HTTPServer resource with name "httpserver-demo"
egctl delete httpserver --all                  # delete all HTTPServer resources
egctl delete customdatakind cdk-demo cdk-kind  # delete CustomDataKind resources named "cdk-demo" and "cdk-kind"
```

## Other commands
```
egctl api-resources                    # view all available resources 
egctl completion zsh                   # generate completion script for zsh
egctl health                           # check easegress health

egctl profile info                     # show location of profile files
egctl profile start cpu ./cpu-profile  # start the CPU profile and store the output in the ./cpu-profile file
egctl profile stop                     # stop profile
```

## Config

By default, `egctl` searches for a file named `.egctlrc` in the `$HOME` directory. Here's an example of a `.egctlrc` file.

```yaml
kind: Config

# current used context.
current-context: context-default

# "contexts" section contains "user" and "cluster" information, which informs egctl about which "user" should be used to access a specific "cluster".
contexts:
  - context:
      cluster: cluster-default
      user: user-default
    name: context-default

# "clusters" section contains information about the "cluster".
# "server" specifies the host address that egctl should access.
# "certificate-authority" or "certificate-authority-data" contain the root certificate authority that the client uses to verify server certificates.
clusters:
  - cluster:
      server: localhost:2381
      certificate-authority: "/tmp/certs/ca.crt" 
      certificate-authority-data: "xxxx"
    name: cluster-default

# "users" section contains "user" information.
# "username" and "password" are used for basic authentication.
# either the pair ("client-key", "client-certificate") or the pair ("client-key-data", "client-certificate-data") contains the client certificate.
users:
  - name: user-default
    user:
      username: user123
      password: password
      client-certificate: "/tmp/certs/client.crt"
      client-key: "/tmp/certs/client.key"
```

To utilize basic authentication with `username` and `password`, add the following segment to the Easegress configuration YAML file:

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

To verify the client certificate, incorporate the following section into the Easegress configuration YAML file:

```yaml
name: easegress-1
cluster:
  listen-peer-urls:
  - http://localhost:2380
  ...
...
# providing a non-empty client-ca-file will force the server to verify the client certificate.
client-ca-file: "/tmp/certs/ca.crt"
```

```
egctl config current-context     # display the current context in use by egctl
egctl config get-contexts        # view all available contexts
egctl config use-context <name>  # update the current-context field in the .egctlrc file to <name>
egctl config view                # display the contents of the configuration file
```
