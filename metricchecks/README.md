# metric checker

##Dependencies
Package dependency is managed by godep (https://github.com/tools/godep). Follow the docs there when adding/removing/updating
package dependencies.

##example api usage

```
import "github.com/square/inspect/metricchecks"
import "github.com/square/inspect/metrics"

m := metrics.NewMetricContext("test")

... //Collect metrics

checker := metricchecks.New()
checker.NewScopeAndPackage() //reset scope to check new set of metrics
checker.InsertMetricValuesFromContext(m) //collect metric values

results, err := checker.CheckAll() //get check results
```


##example command line tool usage

`./metric_checker -address localhost:12345 -cnf ./inspect.confi`

## configuration file format
```
# example comment line

#each section name should be unique
[section name]

#expr indicates the expression to be evaluated
expr = value >= 30

#true indicates the message to send if expr evaluates to true
true = check passed

#false indicates the message to send if expr evaluates to false
false = check failed

#val indicates the main metric that is being checked
val = example_metric_name_val

#tags indicates the main owners/people/services responsible for responding to this metric. csv format
tags = person@example.com,person2@example.com

[another example]
expr = mysqlstat_somemetric_value < 10
true = check failed
val = mysqlstat_somemetric_value
tags = someone@

# true or false are not required in each section, there will just
# be no output if the expr evaluates to the missing option
```
Currently, metric gauges' values are accessed by `metric_name_value`. 
Counter values are accessed with `metric_name_current`.
Counter rates are accessed with `metric_name_rate`.
