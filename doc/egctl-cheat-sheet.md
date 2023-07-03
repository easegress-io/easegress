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