package nats

import (
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redpanda-data/benthos/v4/public/service"
)

var _ = Describe("NATS Error Handler", func() {
	var (
		mgr *service.Resources
	)

	BeforeEach(func() {
		mgr = service.MockResources()
	})

	Describe("errorHandlerOption", func() {
		It("should create error handler option", func() {
			logger := mgr.Logger()
			option := errorHandlerOption(logger)
			Expect(option).NotTo(BeNil())

			// Create options and apply the error handler
			opts := &nats.Options{}
			err := option(opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.AsyncErrorCB).NotTo(BeNil())
		})

		It("should handle error with nil connection and subscription", func() {
			logger := mgr.Logger()
			option := errorHandlerOption(logger)

			// Create options and apply the error handler
			opts := &nats.Options{}
			err := option(opts)
			Expect(err).NotTo(HaveOccurred())

			// Call the error handler with nil values - should not panic
			testErr := errors.New("test error")
			opts.AsyncErrorCB(nil, nil, testErr)
		})
	})
})

func TestNATSErrors(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NATS Errors Suite")
}