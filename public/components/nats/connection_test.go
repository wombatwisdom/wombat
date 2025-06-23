package nats

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/nats-io/nats.go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redpanda-data/benthos/v4/public/service"
)

var _ = Describe("NATS Connection", func() {
	var (
		mgr  *service.Resources
		spec *service.ConfigSpec
	)

	BeforeEach(func() {
		mgr = service.MockResources()
		spec = service.NewConfigSpec()
		for _, field := range connectionHeadFields() {
			spec = spec.Field(field)
		}
		for _, field := range connectionTailFields() {
			spec = spec.Field(field)
		}
	})

	Describe("connectionDetailsFromParsed", func() {
		Context("with valid configuration", func() {
			It("should parse single URL", func() {
				confStr := `
urls: ["nats://localhost:4222"]
`
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				connDetails, err := connectionDetailsFromParsed(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(connDetails.urls).To(Equal("nats://localhost:4222"))
			})

			It("should parse multiple URLs", func() {
				confStr := `
urls: 
  - "nats://localhost:4222"
  - "nats://localhost:4223"
  - "nats://localhost:4224"
`
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				connDetails, err := connectionDetailsFromParsed(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(connDetails.urls).To(Equal("nats://localhost:4222,nats://localhost:4223,nats://localhost:4224"))
			})

			It("should parse URLs with authentication", func() {
				confStr := `
urls: ["nats://username:password@localhost:4222"]
`
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				connDetails, err := connectionDetailsFromParsed(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(connDetails.urls).To(Equal("nats://username:password@localhost:4222"))
			})

			It("should parse without TLS when not specified", func() {
				confStr := `
urls: ["nats://localhost:4222"]
`
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				connDetails, err := connectionDetailsFromParsed(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(connDetails.tlsConf).To(BeNil())
			})

			It("should parse with TLS enabled", func() {
				confStr := `
urls: ["nats://localhost:4222"]
tls:
  enabled: true
`
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				connDetails, err := connectionDetailsFromParsed(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(connDetails.tlsConf).NotTo(BeNil())
				// Default TLS config should have InsecureSkipVerify set to false
				Expect(connDetails.tlsConf.InsecureSkipVerify).To(BeFalse())
			})

			It("should parse with TLS skip verify", func() {
				confStr := `
urls: ["nats://localhost:4222"]
tls:
  enabled: true
  skip_cert_verify: true
`
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				connDetails, err := connectionDetailsFromParsed(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(connDetails.tlsConf).NotTo(BeNil())
				Expect(connDetails.tlsConf.InsecureSkipVerify).To(BeTrue())
			})

			It("should parse with TLS disabled explicitly", func() {
				confStr := `
urls: ["nats://localhost:4222"]
tls:
  enabled: false
`
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				connDetails, err := connectionDetailsFromParsed(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(connDetails.tlsConf).To(BeNil())
			})

			It("should parse with nkey authentication", func() {
				confStr := `
urls: ["nats://localhost:4222"]
auth:
  nkey_file: "./test.nk"
`
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				connDetails, err := connectionDetailsFromParsed(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(connDetails.authConf.NKeyFile).To(Equal("./test.nk"))
			})

			It("should parse with user credentials file", func() {
				confStr := `
urls: ["nats://localhost:4222"]
auth:
  user_credentials_file: "./user.creds"
`
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				connDetails, err := connectionDetailsFromParsed(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(connDetails.authConf.UserCredentialsFile).To(Equal("./user.creds"))
			})

			It("should parse with user JWT and nkey seed", func() {
				confStr := `
urls: ["nats://localhost:4222"]
auth:
  user_jwt: "eyJ0eXAiOiJqd3QiLCJhbGciOiJlZDI1NTE5In0.test"
  user_nkey_seed: "SUACSSL3UAHUDXKFSNVUZRF5UHPMWZ6BFDTJ7M6USDXIEDNPPQYYYCU3VY"
`
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				connDetails, err := connectionDetailsFromParsed(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				Expect(connDetails.authConf.UserJWT).To(Equal("eyJ0eXAiOiJqd3QiLCJhbGciOiJlZDI1NTE5In0.test"))
				Expect(connDetails.authConf.UserNkeySeed).To(Equal("SUACSSL3UAHUDXKFSNVUZRF5UHPMWZ6BFDTJ7M6USDXIEDNPPQYYYCU3VY"))
			})

			It("should set label from resources", func() {
				// MockResources creates a resources with an empty label by default
				confStr := `
urls: ["nats://localhost:4222"]
`
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				connDetails, err := connectionDetailsFromParsed(conf, mgr)
				Expect(err).NotTo(HaveOccurred())
				// Default empty label
				Expect(connDetails.label).To(Equal(""))
			})
		})

		Context("with invalid configuration", func() {
			It("should fail without urls", func() {
				confStr := ``
				_, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).To(HaveOccurred())
			})

			It("should fail with invalid auth configuration", func() {
				confStr := `
urls: ["nats://localhost:4222"]
auth:
  user_jwt: "some-jwt"
  # Missing user_nkey_seed
`
				conf, err := spec.ParseYAML(confStr, service.NewEnvironment())
				Expect(err).NotTo(HaveOccurred())

				_, err = connectionDetailsFromParsed(conf, mgr)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("missing auth.user_nkey_seed"))
			})
		})
	})

	Describe("connectionDetails.get", func() {
		var connDetails connectionDetails

		BeforeEach(func() {
			connDetails = connectionDetails{
				label:  "test-connection",
				logger: mgr.Logger(),
				urls:   "nats://localhost:4222",
				fs:     mgr.FS(),
			}
		})

		It("should build connection options without TLS", func() {
			connDetails.tlsConf = nil
			
			// Since we can't actually connect in unit tests, we'll verify the behavior
			// by checking that get() returns an error when trying to connect to invalid URL
			_, err := connDetails.get(context.Background())
			Expect(err).To(HaveOccurred())
			// The error should be a connection error, not a configuration error
			Expect(err.Error()).To(ContainSubstring("nats://localhost:4222"))
		})

		It("should build connection options with TLS", func() {
			connDetails.tlsConf = &tls.Config{
				InsecureSkipVerify: true,
			}
			
			_, err := connDetails.get(context.Background())
			Expect(err).To(HaveOccurred())
			// The error should be a connection error, not a configuration error
			Expect(err.Error()).To(ContainSubstring("nats://localhost:4222"))
		})

		It("should include extra options", func() {
			extraOptApplied := false
			extraOpt := func(o *nats.Options) error {
				extraOptApplied = true
				return nil
			}
			
			_, err := connDetails.get(context.Background(), extraOpt)
			Expect(err).To(HaveOccurred())
			Expect(extraOptApplied).To(BeTrue())
		})
	})
})

func TestNATSConnection(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NATS Connection Suite")
}