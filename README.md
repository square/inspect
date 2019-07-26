inspect is a collection of operating system/application monitoring
analysis libraries and utilities with an emphasis on problem detection.

#### Installation
  1. get go
  2. go get -u -v github.com/square/inspect/...

The above commands should install three binaries in your original $GOPATH/bin directory.

1. inspect 
2. inspect-mysql (work in progress)
3. inspect-postgres (work in progress)

Please see subdirectories for more detailed documentation

#### Glossary
* cmd - Directory for command line programs based on the below libraries

<img src="https://raw.githubusercontent.com/square/inspect/master/cmd/inspect/screenshots/summary.png" height="259" width="208">

* os      - Operating system metric measurement libraries used by inspect.
* mysql   - MySQL metric reporting libraries.
* postgres  - Postgres metric reporting libraries.
* metrics/metricscheck - Simple metrics libraries for golang.

#### Development

Development setup is a bit tricky given interaction of godep/gopath:
* Create a fork
* Setup golang workspace and set GOPATH [Reference](https://golang.org/doc/code.html#Workspaces)
  * export GOPATH=$HOME/godev # example
  * mkdir -p $GOPATH/{src,bin,pkg}
* Setup project
  * mkdir -p $GOPATH/src/github.com/square
  * cd $GOPATH/src/github.com/square
  * git clone git@github.com:CHANGE-ME/inspect.git # change path to your fork
  * cd inspect
* Setup a reference to upstream to sync changes with upstream easily etc
  * git remote add upstream github.com/square/inspect.git
```
[s@pain inspect (master)]$ git remote -v
origin	git@github.com:syamp/inspect.git (fetch)
origin	git@github.com:syamp/inspect.git (push)
upstream	github.com/square/inspect.git (fetch)
upstream	github.com/square/inspect.git (push)
```
* We use godep for vendoring and dependency management. We rewrite import
  paths. If you are adding a new dependency or updating one, please run
  1. godep save -r
  
* Please format, test and lint before submitting PRs
  1. go fmt ./...
  2. go test ./...
  3. $GOPATH/bin/golint ./...

#### Todo
* metriccheck uses some darkmagic and uses golang/x/tools APIs which tend to break API compat often. Need to fix it.
