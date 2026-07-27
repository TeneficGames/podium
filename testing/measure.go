package testing

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const defaultMeasureLoops = 5

type measureFlag int

const (
	measureFlagNone measureFlag = iota
	measureFlagFocused
	measureFlagPending
)

// HTTPMeasure runs the specified specs in an http test
func HTTPMeasure(description string, setup func(map[string]interface{}), f func(string, map[string]interface{}), timeout float64) bool {
	return measure(description, setup, f, timeout, measureFlagNone)
}

// FHTTPMeasure runs the specified specs in an http test
func FHTTPMeasure(description string, setup func(map[string]interface{}), f func(string, map[string]interface{}), timeout float64) bool {
	return measure(description, setup, f, timeout, measureFlagFocused)
}

// XHTTPMeasure runs the specified specs in an http test
func XHTTPMeasure(description string, setup func(map[string]interface{}), f func(string, map[string]interface{}), timeout float64) bool {
	return measure(description, setup, f, timeout, measureFlagPending)
}

func measure(description string, setup func(map[string]interface{}), f func(string, map[string]interface{}), timeout float64, flag measureFlag) bool {
	app := GetDefaultTestApp()

	d := ginkgo.Describe
	if flag == measureFlagFocused {
		d = ginkgo.FDescribe
	}
	if flag == measureFlagPending {
		d = ginkgo.XDescribe
	}

	d("Measure", ginkgo.Label("performance"), func() {
		var ctx map[string]interface{}

		BeforeOnce(func() {
			InitializeTestServer(app)
			ctx = map[string]interface{}{"app": app}
			setup(ctx)
		})

		ginkgo.AfterEach(func() {
			transport.CloseIdleConnections()
		})

		ginkgo.It(description, func() {
			for i := 0; i < measureLoopCount(); i++ {
				start := time.Now()
				f(app.HTTPEndpoint, ctx)
				runtime := time.Since(start)
				Expect(runtime.Seconds()).Should(
					BeNumerically("<", timeout),
					fmt.Sprintf("%s shouldn't take too long.", description),
				)
			}
		})
	})

	return true
}

func measureLoopCount() int {
	loops, err := strconv.Atoi(os.Getenv("PODIUM_MEASURE_LOOPS"))
	if err != nil || loops < 1 {
		return defaultMeasureLoops
	}
	return loops
}
