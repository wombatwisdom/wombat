package nats

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redpanda-data/benthos/v4/public/service"
)

var _ = Describe("JetStream Output Config", func() {
	var mgr *service.Resources

	BeforeEach(func() {
		mgr = service.MockResources()
	})

	Describe("newJetStreamWriterFromConfig", func() {
		It("should create writer with basic configuration", func() {
			confStr := `
urls: ["nats://localhost:4222"]
subject: test.subject
`
			spec := natsJetStreamOutputConfig()
			conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
			Expect(err).NotTo(HaveOccurred())

			writer, err := newJetStreamWriterFromConfig(conf, mgr)
			Expect(err).NotTo(HaveOccurred())
			Expect(writer).NotTo(BeNil())
			Expect(writer.subjectStrRaw).To(Equal("test.subject"))
		})

		It("should create writer with interpolated subject", func() {
			confStr := `
urls: ["nats://localhost:4222"]
subject: 'foo.${! json("type") }.bar'
`
			spec := natsJetStreamOutputConfig()
			conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
			Expect(err).NotTo(HaveOccurred())

			writer, err := newJetStreamWriterFromConfig(conf, mgr)
			Expect(err).NotTo(HaveOccurred())
			Expect(writer.subjectStr).NotTo(BeNil())
		})

		It("should create writer with headers", func() {
			confStr := `
urls: ["nats://localhost:4222"]
subject: test.subject
headers:
  Content-Type: application/json
  X-Custom: '${! meta("custom_value") }'
`
			spec := natsJetStreamOutputConfig()
			conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
			Expect(err).NotTo(HaveOccurred())

			writer, err := newJetStreamWriterFromConfig(conf, mgr)
			Expect(err).NotTo(HaveOccurred())
			Expect(writer.headers).To(HaveLen(2))
			Expect(writer.headers).To(HaveKey("Content-Type"))
			Expect(writer.headers).To(HaveKey("X-Custom"))
		})

		It("should create writer with metadata filter", func() {
			confStr := `
urls: ["nats://localhost:4222"]
subject: test.subject
metadata:
  include_patterns: ["foo_*", "bar_*"]
`
			spec := natsJetStreamOutputConfig()
			conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
			Expect(err).NotTo(HaveOccurred())

			writer, err := newJetStreamWriterFromConfig(conf, mgr)
			Expect(err).NotTo(HaveOccurred())
			Expect(writer.metaFilter).NotTo(BeNil())
		})

		It("should fail without subject", func() {
			confStr := `
urls: ["nats://localhost:4222"]
`
			spec := natsJetStreamOutputConfig()
			_, err := spec.ParseYAML(confStr, service.NewEnvironment())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("convertMsg", func() {
		var writer *jetStreamOutput

		BeforeEach(func() {
			writer = &jetStreamOutput{
				log:           mgr.Logger(),
				subjectStrRaw: "test.subject",
			}
			subjectStr, _ := service.NewInterpolatedString(writer.subjectStrRaw)
			writer.subjectStr = subjectStr
			writer.headers = make(map[string]*service.InterpolatedString)
		})

		It("should convert basic message", func() {
			msg := service.NewMessage([]byte("test message"))

			natsMsg, err := writer.convertMsg(msg)
			Expect(err).NotTo(HaveOccurred())
			Expect(natsMsg.Subject).To(Equal("test.subject"))
			Expect(natsMsg.Data).To(Equal([]byte("test message")))
		})

		It("should add static headers", func() {
			contentType, _ := service.NewInterpolatedString("application/json")
			writer.headers["Content-Type"] = contentType

			msg := service.NewMessage([]byte("test message"))

			natsMsg, err := writer.convertMsg(msg)
			Expect(err).NotTo(HaveOccurred())
			Expect(natsMsg.Header.Get("Content-Type")).To(Equal("application/json"))
		})

		It("should interpolate headers", func() {
			customHeader, _ := service.NewInterpolatedString("${! meta(\"request_id\") }")
			writer.headers["X-Request-ID"] = customHeader

			msg := service.NewMessage([]byte("test message"))
			msg.MetaSet("request_id", "12345")

			natsMsg, err := writer.convertMsg(msg)
			Expect(err).NotTo(HaveOccurred())
			Expect(natsMsg.Header.Get("X-Request-ID")).To(Equal("12345"))
		})

		It("should handle subject interpolation error", func() {
			subjectStr, _ := service.NewInterpolatedString("test.${! json(\"missing\") }")
			writer.subjectStr = subjectStr

			msg := service.NewMessage([]byte("not json"))

			_, err := writer.convertMsg(msg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed string interpolation on field \"subject\""))
		})

		It("should handle header interpolation error", func() {
			customHeader, _ := service.NewInterpolatedString("${! json(\"missing\") }")
			writer.headers["X-Bad-Header"] = customHeader

			msg := service.NewMessage([]byte("not json"))

			_, err := writer.convertMsg(msg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed string interpolation on header \"X-Bad-Header\""))
		})
	})
})

// Tests are run via the main nats_suite_test.go