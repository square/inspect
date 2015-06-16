[![Build Status](https://travis-ci.org/square/inspect.svg?branch=master)](https://travis-ci.org/square/inspect)


inspect is a collection of operating system/application monitoring
analysis libraries and utilities with an emphasis on problem detection.

#### Installation
  1. get go
  2. go get -u -v github.com/tools/godep
  3. git clone https://github.com/square/inspect.git
  4. cd inspect
  5. GOPATH=`$GOPATH/bin/godep path`:$GOPATH
  6. go install ./...
  
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
* We use godep for vendoring and dependency management.
  1. godep restore # restore to last known good set
* Please run gofmt and golint before submitting PRs
  1. go fmt ./...
  2. go test ./...
  3. $GOPATH/bin/golint ./...

#### Todo
* metriccheck uses some darkmagic and uses golang/x/tools APIs which tend to break API compat often. Need to fix it.
