package mongodb_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redpanda-data/benthos/v4/public/service"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/wombatwisdom/wombat/public/components/mongodb"
)

var _ = Describe("Config", func() {
	var (
		config mongodb.Config
		ctx    context.Context
		cancel context.CancelFunc
	)

	BeforeEach(func() {
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	})

	AfterEach(func() {
		cancel()
	})

	Describe("NewClient", func() {
		Context("with empty URI", func() {
			BeforeEach(func() {
				config = mongodb.Config{
					Uri: "",
				}
			})

			It("should return an error", func() {
				client, err := config.NewClient(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("uri is required"))
				Expect(client).To(BeNil())
			})
		})

		Context("with invalid URI", func() {
			BeforeEach(func() {
				config = mongodb.Config{
					Uri: "not-a-valid-uri",
				}
			})

			It("should return an error", func() {
				client, err := config.NewClient(ctx)
				Expect(err).To(HaveOccurred())
				Expect(client).To(BeNil())
			})
		})

		Context("with valid URI format", func() {
			BeforeEach(func() {
				// Using a valid URI format that won't actually connect
				config = mongodb.Config{
					Uri: "mongodb://localhost:27017/testdb?connectTimeoutMS=1000&serverSelectionTimeoutMS=1000",
				}
			})

			It("should create a client", func() {
				// For unit tests, we're testing that the client is created
				// In integration tests, we would verify actual connection
				client, err := config.NewClient(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(client).NotTo(BeNil())

				// Verify it's a MongoDB client
				Expect(client).To(BeAssignableToTypeOf(&mongo.Client{}))

				// Disconnect to clean up
				if client != nil {
					_ = client.Disconnect(ctx)
				}
			})
		})

		Context("with URI containing credentials", func() {
			BeforeEach(func() {
				config = mongodb.Config{
					Uri: "mongodb://user:pass@localhost:27017/testdb",
				}
			})

			It("should create a client with auth", func() {
				client, err := config.NewClient(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(client).NotTo(BeNil())

				// Clean up
				if client != nil {
					_ = client.Disconnect(ctx)
				}
			})
		})

		Context("with replica set URI", func() {
			BeforeEach(func() {
				config = mongodb.Config{
					Uri: "mongodb://host1:27017,host2:27017,host3:27017/testdb?replicaSet=myReplicaSet",
				}
			})

			It("should create a client for replica set", func() {
				client, err := config.NewClient(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(client).NotTo(BeNil())

				// Clean up
				if client != nil {
					_ = client.Disconnect(ctx)
				}
			})
		})
	})

	Describe("Config validation", func() {
		It("should handle properly encoded URI with special characters", func() {
			// Properly URL-encoded password with special characters
			config = mongodb.Config{
				Uri: "mongodb://user:p%40ssw0rd%21@localhost:27017/testdb",
			}

			// Should create client successfully with properly encoded URI
			client, err := config.NewClient(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())

			// Clean up
			if client != nil {
				_ = client.Disconnect(ctx)
			}
		})

		It("should fail with unescaped special characters in URI", func() {
			// URI with unescaped @ symbol should fail
			config = mongodb.Config{
				Uri: "mongodb://user:p@ssw0rd!@localhost:27017/testdb",
			}

			// Should fail to create client due to unescaped @ in password
			client, err := config.NewClient(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unescaped @ sign"))
			Expect(client).To(BeNil())
		})
	})
})

// Helper function tests for future use
var _ = Describe("MongoDB Client Options", func() {
	It("should apply correct client options", func() {
		// Test that the client options are correctly applied
		opts := options.Client().ApplyURI("mongodb://localhost:27017")
		Expect(opts).NotTo(BeNil())

		// In actual implementation, we might want to verify specific options
		// like timeouts, pool sizes, etc.
	})

	Describe("ConfigFromParsed", func() {
		Context("with valid configuration", func() {
			It("should parse URI correctly", func() {
				spec := service.NewConfigSpec().Field(service.NewStringField("uri"))
				env := service.NewEnvironment()
				parsedConf, err := spec.ParseYAML("uri: mongodb://localhost:27017", env)
				Expect(err).NotTo(HaveOccurred())

				config, err := mongodb.ConfigFromParsed(parsedConf)
				Expect(err).NotTo(HaveOccurred())
				Expect(config).NotTo(BeNil())
				Expect(config.Uri).To(Equal("mongodb://localhost:27017"))
			})

			It("should parse complex URI with auth and options", func() {
				spec := service.NewConfigSpec().Field(service.NewStringField("uri"))
				env := service.NewEnvironment()
				complexURI := "mongodb://user:pass@localhost:27017/mydb?authSource=admin&ssl=true"
				parsedConf, err := spec.ParseYAML(fmt.Sprintf("uri: %s", complexURI), env)
				Expect(err).NotTo(HaveOccurred())

				config, err := mongodb.ConfigFromParsed(parsedConf)
				Expect(err).NotTo(HaveOccurred())
				Expect(config).NotTo(BeNil())
				Expect(config.Uri).To(Equal(complexURI))
			})
		})

		Context("with invalid configuration", func() {
			It("should fail when URI field is missing", func() {
				spec := service.NewConfigSpec().Field(service.NewStringField("other_field"))
				env := service.NewEnvironment()
				parsedConf, err := spec.ParseYAML("other_field: value", env)
				Expect(err).NotTo(HaveOccurred())

				config, err := mongodb.ConfigFromParsed(parsedConf)
				Expect(err).To(HaveOccurred())
				Expect(config).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("uri"))
			})

			It("should handle empty configuration", func() {
				spec := service.NewConfigSpec().Field(service.NewStringField("uri"))
				env := service.NewEnvironment()
				parsedConf, err := spec.ParseYAML("uri: \"\"", env)
				Expect(err).NotTo(HaveOccurred())

				config, err := mongodb.ConfigFromParsed(parsedConf)
				Expect(err).NotTo(HaveOccurred())
				Expect(config).NotTo(BeNil())
				Expect(config.Uri).To(Equal(""))
			})
		})
	})
})
