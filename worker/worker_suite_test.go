package worker_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = BeforeSuite(func() {
	Expect(os.Setenv("PODIUM_REDIS_DB", "1")).To(Succeed())
})

var _ = AfterSuite(func() {
	Expect(os.Unsetenv("PODIUM_REDIS_DB")).To(Succeed())
})

func TestApi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Worker Suite")
}
