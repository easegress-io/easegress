# egbuilder <!-- omit in toc -->
`egbuilder` is a command-line tool that enables you to create, build, and run Easegress with custom plugins.


- [Init a custom plugin project](#init-a-custom-plugin-project)
- [Add more plugins](#add-more-plugins)
- [Build Easegress with custom plugins](#build-easegress-with-custom-plugins)
- [Run Easegress in current directory](#run-easegress-in-current-directory)
- [Environment variables](#environment-variables)

## Init a custom plugin project
The `egbuilder init` command helps initialize a custom plugin project, creating the necessary directories and files for users to get started.

```
egbuilder init --repo github.com/your/repo \
    --filters=MyFilter1,MyFilter2 \
    --controllers=MyController1,MyController2
```

The example above will create following directories and files.
```
.
├── .egbuilderrc
├── controllers
│   ├── mycontroller1
│   │   └── mycontroller1.go
│   └── mycontroller2
│       └── mycontroller2.go
├── filters
│   ├── myfilter1
│   │   └── myfilter1.go
│   └── myfilter2
│       └── myfilter2.go
├── go.mod
├── go.sum
└── registry
    └── registry.go
```
The `.egbuilderrc` file is a configuration file that can be used by the `egbuilder add` and `egbuilder run` commands. The `registry/registry.go` file contains code generated to register custom filters and controllers with Easegress. The `controllers` and `filters` directories contain the necessary variables, structures, and methods to get started.

## Add more plugins
The `egbuilder add` command allows you to add more custom filters and controllers to an existing custom plugin project.

```
egbuilder add --filters=MyFilter3,MyFilter4 \ 
    --controllers=MyController3,MyController4
```

The above example will add following directories and files.
```
.
├── controllers
    ...
│   ├── mycontroller3
│   │   └── mycontroller3.go
│   └── mycontroller4
│       └── mycontroller4.go
├── filters
    ...
│   ├── myfilter3
│   │   └── myfilter3.go
│   └── myfilter4
│       └── myfilter4.go
...
```
The `.egbuilderrc` and `registry/registry.go` files will be updated based on changes.

## Build Easegress with custom plugins
The `egbuilder build` command is used to compile Easegress with custom plugins.

```
egbuilder build -f build.yaml
```
where `build.yaml` contains:
```yaml
# egVersion: the version of Easegress used for building. Supports versions v2.5.2 and later.
# An empty egVersion value means using the latest version of Easegress.
egVersion: v2.5.2

# plugins: custom plugins.
# It is recommended to use plugins created with "egbuilder init".
# Generally, any plugin containing "registry/registry.go" can utilize the "egbuilder build" command.
# You can initialize a project to see for yourself.
plugins:
- module: github.com/your/repo
  version: ""
  replacement: "."
- module: github.com/other/repo

# output: path of output file.
output: "./easegress-server"

# raceDetector: "-race" flag for go build
raceDetector: false

# skipBuild: if true, causes egbuilder to not compile the program, it is used in conjunction with build tools such as GoReleaser. Implies skipCleanUp to be true.
skipBuild: false

# skipCleanup: if true, not clean up the temp directory after exiting.
skipCleanup: false

# buildFlags: flags for "go build" command
buildFlags: []

# modFlags: flags for "go mod" command
modFlags: []

# compile: GOOS, GOARCH, GOARM env variable for "go build"
compile:
  os: ""
  arch: ""
  arm: ""
  cgo: false
```

## Run Easegress in current directory
The `egbuilder run` command is used to run Easegress with custom plugins in current working directory.

```bash
egbuilder run              # run with default
egbuilder run -f run.yaml  # run with config
```
where `run.yaml` contains:

```yaml
egVersion: v2.5.2

# egServerArgs: args for easegress-server
# egServerArgs: ["--config-file", "easegress-server.yaml"] means
# ./easegress-server --config-file easegress-server.yaml
egServerArgs: []

raceDetector: false
skipBuild: false
skipCleanup: false
buildFlags: []
modFlags: []
compile:
  os: ""
  arch: ""
  arm: ""
  cgo: false
```

So, `egbuilder run` can be seen as executing `egbuilder build` first, followed by running `./easegress-server`.

## Environment variables
- `EGBUILDER_GO` sets the go command to use when more then one version of go is installed.
