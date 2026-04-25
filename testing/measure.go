package testing

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const measureLoops = 200

type measureFlag int

const (
	measureFlagNone measureFlag = iota
	measureFlagFocused
	measureFlagPending
)

//HTTPMeasure runs the specified specs in an http test
func HTTPMeasure(description string, setup func(map[string]interface{}), f func(string, map[string]interface{}), timeout float64) bool {
	return measure(description, setup, f, timeout, measureFlagNone)
}

//FHTTPMeasure runs the specified specs in an http test
func FHTTPMeasure(description string, setup func(map[string]interface{}), f func(string, map[string]interface{}), timeout float64) bool {
	return measure(description, setup, f, timeout, measureFlagFocused)
}

//XHTTPMeasure runs the specified specs in an http test
func XHTTPMeasure(description string, setup func(map[string]interface{}), f func(string, map[string]interface{}), timeout float64) bool {
	return measure(description, setup, f, timeout, measureFlagPending)
}

func measure(description string, setup func(map[string]interface{}), f func(string, map[string]interface{}), timeout float64, flag measureFlag) bool {
	app := GetDefaultTestApp()

	d := func(t string, f func()) { ginkgo.Describe(t, f) }
	if flag == measureFlagFocused {
		d = func(t string, f func()) { ginkgo.FDescribe(t, f) }
	}
	if flag == measureFlagPending {
		d = func(t string, f func()) { ginkgo.XDescribe(t, f) }
	}

	d("Measure", func() {
		var loops int
		var ctx map[string]interface{}

		BeforeOnce(func() {
			InitializeTestServer(app)
			ctx = map[string]interface{}{"app": app}
			setup(ctx)
		})

		ginkgo.AfterEach(func() {
			loops++
			if loops == measureLoops {
				transport.CloseIdleConnections()
			}
		})

		ginkgo.It(description, func() {
			for i := 0; i < measureLoops; i++ {
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
