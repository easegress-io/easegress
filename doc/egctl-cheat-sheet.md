# egctl Cheat Sheet

## Creating resources
Easegress manifests are defined in YAML. The file extension .yaml and .yml be used. Use `egctl create <resource>`, or `egctl apply <resource>` to create resources. Use `egctl api-resources` to view all available resources and their supported actions. 

```
egctl apply object -f ./pipeline-demo.yaml     # create resource object
egctl create object -f ./httpserver-demo.yaml  # create resource object

egctl apply customdatakind -f ./cdk-demo.yaml              # create resource customdatakind
egctl create customdata cdk-demo -f custom-data-demo.yaml  # create resource customdata
```

## Viewing and finding resources 

```
egctl get objects                 # list all objects
egctl get object httpserver-demo  # find object httpserver-demo
egctl get objectkinds             # list all available object kinds
egctl get objectstatus            # list all object status

egctl get member                  # list all easegress nodes
egctl get member eg-default-name  # find easegress node with name eg-default-name

egctl get customdatakind          # list all custom data kind
egctl get customdata cdk-demo     # find custom data kind cdk-demo
 
egctl describe objects               # describe all objects 
egctl describe object pipeline-demo  # describe object pipeline-demo
```

## Updating resources
```
egctl apply object -f httpserver-demo-version2.yaml  # update object with new yaml file.
egctl apply customdatakind -f cdk-demo2.yaml         # udpate custom data kind with new yaml file.
```

## Deleting resources
```
egctl delete object httpserver-demo   # delete object httpserver-demo
egctl delete object --all             # delete all objects
egctl delete customdatakind cdk-demo  # delete custom data kind cdk-demo
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