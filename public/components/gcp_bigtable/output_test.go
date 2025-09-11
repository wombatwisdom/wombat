package gcp_bigtable_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redpanda-data/benthos/v4/public/service"

	_ "github.com/wombatwisdom/wombat/public/components/all"
)

var _ = Describe("Output", func() {
	var tableName string
	var config string

	BeforeEach(func() {
		// create a test table
		tableName = fmt.Sprintf("test-table-%s", uuid.NewString())
		err := btac.CreateTable(context.Background(), tableName)
		Expect(err).NotTo(HaveOccurred())

		err = btac.CreateColumnFamily(context.Background(), tableName, "general")
		Expect(err).NotTo(HaveOccurred())

		err = btac.CreateColumnFamily(context.Background(), tableName, "invoices")
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
      root.key = "my/test/key"
      root.general.name = "Daan Gerits"
      root.general.age = 25
      root.invoices.INV001.amount = 100
      root.invoices.INV001.date = "2021-01-01"
      root.invoices.INV002.amount = 200
      root.invoices.INV002.date = "2021-01-02"

output:
  gcp_bigtable:
    emulated_host_port: %s
    project: fake-project
    instance: fake-instance
    table: %s
    key: this.key
    data: this.without("key")
`, srv.Addr, tableName))
	})

	It("should write the data to bigtable", func() {
		sb := service.NewStreamBuilder()
		Expect(sb.SetYAML(config)).To(Succeed())

		stream, err := sb.Build()
		Expect(err).NotTo(HaveOccurred())

		err = stream.Run(context.Background())
		Expect(err).NotTo(HaveOccurred())

		tbl := btc.Open(tableName)
		row, err := tbl.ReadRow(context.Background(), "my/test/key")
		Expect(err).NotTo(HaveOccurred())

		Expect(row.Key()).To(Equal("my/test/key"))

		general := row["general"]
		Expect(general).To(HaveLen(2))

		ri := general[0]
		Expect(ri.Column).To(Equal("general:age"))
		Expect(ri.Value).To(Equal([]byte("25")))

		ri = general[1]
		Expect(ri.Column).To(Equal("general:name"))
		Expect(ri.Value).To(Equal([]byte("\"Daan Gerits\"")))

		invoices := row["invoices"]
		Expect(invoices).To(HaveLen(2))

		ri = invoices[0]
		Expect(ri.Column).To(Equal("invoices:INV001"))
		Expect(ri.Value).To(Equal([]byte(`{"amount":100,"date":"2021-01-01"}`)))

		ri = invoices[1]
		Expect(ri.Column).To(Equal("invoices:INV002"))
		Expect(ri.Value).To(Equal([]byte(`{"amount":200,"date":"2021-01-02"}`)))
	})
})
