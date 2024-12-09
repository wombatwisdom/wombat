package gcp_bigtable_test

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redpanda-data/benthos/v4/public/service"
	"strings"

	_ "github.com/wombatwisdom/wombat/public/components/all"
)

var _ = Describe("Output", func() {
	var tableName string
	var config string

	BeforeEach(func() {
		// create a test table
		tableName = fmt.Sprintf("test-table-%s", uuid.NewString())
		err := btac.CreateTable(context.Background(), "test-table")
		Expect(err).NotTo(HaveOccurred())
	})

	//AfterEach(func() {
	//	// delete the test table
	//	err := btac.DeleteTable(context.Background(), tableName)
	//	Expect(err).NotTo(HaveOccurred())
	//})

	BeforeEach(func() {
		config = strings.TrimSpace(fmt.Sprintf(`
input:
  generate:
    count: 1
    mapping: |
      root.key = uuid_v4()
      root.general.name = "Daan Gerits"
      root.general.age = 25
      root.invoices.INV001.amount = 100
      root.invoices.INV001.date = "2021-01-01"
      root.invoices.INV002.amount = 200
      root.invoices.INV002.date = "2021-01-02"

output:
  gcp_bigtable:
    project: fake-project
    instance: fake-instance
    table: %s
    row_key: this.key
    credentials_json: whatever
`, tableName))
	})

	It("should create a new output", func() {
		sb := service.NewStreamBuilder()
		Expect(sb.SetYAML(config)).To(Succeed())

		stream, err := sb.Build()
		Expect(err).NotTo(HaveOccurred())

		err = stream.Run(context.Background())
		Expect(err).NotTo(HaveOccurred())
	})
})
