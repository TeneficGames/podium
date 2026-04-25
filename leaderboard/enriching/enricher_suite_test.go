package enriching

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEnriching(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Enriching Suite")
}
