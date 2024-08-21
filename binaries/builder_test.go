package binaries

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"os"
	"runtime"
)

var _ = Describe("Builder", func() {
	When("Generating a spec", func() {
		It("should generate files", func() {
			tmpWorkDir, err := os.MkdirTemp("", "wombat-test-")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			//defer os.RemoveAll(tmpWorkDir)

			builder, err := NewBuilder(tmpWorkDir)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			bb := builder.(*baseBuilder)

			vars := templateVars{
				GoVersion:      runtime.Version(),
				BenthosVersion: "v4.35.0",
				DateBuilt:      "2021-09-01",
				Packages: []string{
					"github.com/redpanda-data/connect/v4/public/components/io",
					"github.com/redpanda-data/benthos/v4/public/components/pure",
					"github.com/redpanda-data/benthos/v4/public/components/io",
				},
			}

			err = bb.Generate(vars, tmpWorkDir)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// -- check the go.mod file exists
			_, err = os.Stat(tmpWorkDir + "/go.mod")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// -- check the main.go file exists
			_, err = os.Stat(tmpWorkDir + "/main.go")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			// -- check the contents of the main.go file correspond to the template
			expMainGo := `package main

import (
	"context"
	"github.com/redpanda-data/benthos/v4/public/service"

	_ "github.com/redpanda-data/connect/v4/public/components/io"
	_ "github.com/redpanda-data/benthos/v4/public/components/pure"
	_ "github.com/redpanda-data/benthos/v4/public/components/io"
)

func main() {
	service.RunCLI(context.Background(),
		service.CLIOptSetBinaryName("wombat go"),
		service.CLIOptSetProductName("Wombat"),
		service.CLIOptSetVersion("v4.35.0", "2021-09-01"))
}
`
			mainGo, err := os.ReadFile(tmpWorkDir + "/main.go")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(string(mainGo)).To(gomega.Equal(expMainGo))
		})
	})
})
