package cache

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

func TestEnrichingCache(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Enriching Cache Suite")
}
