package metricchecks

import (
	"net/http"
	"testing"

	"github.com/sorawee/inspect/metrics"

	_ "code.google.com/p/go.tools/go/gcimporter"
	"golang.org/x/tools/go/exact"
	"golang.org/x/tools/go/types"
)

//initializes a test Checker
func initTestChecker() checker {
	c := &checker{
		hostport: "localhost:12345",
	}
	return *c
}

//launches routine to serve json packages
func initMetricsJson() {
	_, err := http.Get("http://localhost:12345/api/v1/metrics.json/")
	if err == nil {
		return
	}
	m := initMetricsContext()
	go func() {
		http.HandleFunc("/api/v1/metrics.json/", m.HttpJsonHandler)
		http.ListenAndServe("localhost:12345", nil)
	}()
	return
}

func initMetricsContext() *metrics.MetricContext {
	m := metrics.NewMetricContext("test")
	g1 := metrics.NewGauge()
	m.Register(g1, "testGauge1")
	g2 := metrics.NewGauge()
	m.Register(g2, "testGauge2")
	g3 := metrics.NewGauge()
	m.Register(g3, "testGauge3")
	g4 := metrics.NewGauge()
	m.Register(g4, "testGauge4")
	g5 := metrics.NewGauge()
	m.Register(g5, "testGauge5")
	g2.Set(float64(200))
	g3.Set(float64(300))
	g4.Set(float64(400))
	g5.Set(float64(500))
	return m
}

func TestNewScopeAndPackage1(t *testing.T) {
	c := initTestChecker()

	c.NewScopeAndPackage()

	if c.sc == nil {
		t.Errorf("scope not set")
	}
	if c.pkg == nil {
		t.Errorf("package not set")
	}
}

func TestNewScopeAndPackage2(t *testing.T) {
	c := initTestChecker()
	c.NewScopeAndPackage()
	c.sc.Insert(types.NewConst(0, c.pkg, "testfloat1",
		types.New("float64"), exact.MakeFloat64(1)))
	c.NewScopeAndPackage()
	for _, name := range c.sc.Names() {
		if name == "testfloat1" {
			t.Errorf("scope not reset")
		}
	}
	o := c.sc.Insert(types.NewConst(0, c.pkg, "testfloat1",
		types.New("float64"), exact.MakeFloat64(2)))
	if o != nil {
		t.Errorf("did not reset scope")
	}
}

func TestInsertMetricValuesFromJSON(t *testing.T) {
	c := initTestChecker()
	c.NewScopeAndPackage()
	initMetricsJson()
	c.InsertMetricValuesFromJSON()
	if f, _ := exact.Float64Val(c.sc.Lookup("testGauge2_value").(*types.Const).Val()); f != float64(200) {
		t.Errorf("Did not insert gauge2 correctly")
	}
	if f, _ := exact.Float64Val(c.sc.Lookup("testGauge3_value").(*types.Const).Val()); f != float64(300) {
		t.Errorf("Did not insert gauge3 correctly")
	}
	if f, _ := exact.Float64Val(c.sc.Lookup("testGauge4_value").(*types.Const).Val()); f != float64(400) {
		t.Errorf("Did not insert gauge4 correctly")
	}
	if f, _ := exact.Float64Val(c.sc.Lookup("testGauge5_value").(*types.Const).Val()); f != float64(500) {
		t.Errorf("Did not insert gauge5 correctly")
	}
}

func TestInsertMetricValuesFromContext(t *testing.T) {
	c := initTestChecker()
	c.NewScopeAndPackage()
	m := initMetricsContext()
	c.InsertMetricValuesFromContext(m)
	if f, _ := exact.Float64Val(c.sc.Lookup("testGauge2_value").(*types.Const).Val()); f != float64(200) {
		t.Errorf("Did not insert gauge2 correctly")
	}
	if f, _ := exact.Float64Val(c.sc.Lookup("testGauge3_value").(*types.Const).Val()); f != float64(300) {
		t.Errorf("Did not insert gauge3 correctly")
	}
	if f, _ := exact.Float64Val(c.sc.Lookup("testGauge4_value").(*types.Const).Val()); f != float64(400) {
		t.Errorf("Did not insert gauge4 correctly")
	}
	if f, _ := exact.Float64Val(c.sc.Lookup("testGauge5_value").(*types.Const).Val()); f != float64(500) {
		t.Errorf("Did not insert gauge5 correctly")
	}
}
