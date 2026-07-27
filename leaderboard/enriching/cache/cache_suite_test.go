package cache

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEnrichingCache(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Enriching Cache Suite")
}
