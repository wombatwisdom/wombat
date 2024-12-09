package gcp_bigtable_test

import (
	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/bigtable/bttest"
	"context"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var srv *bttest.Server
var btc *bigtable.Client
var btac *bigtable.AdminClient

func TestGcpBigtable(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeSuite(func() {
		var err error

		srv, err = bttest.NewServer("localhost:0")
		Expect(err).NotTo(HaveOccurred())

		conn, err := grpc.NewClient(
			srv.Addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		Expect(err).NotTo(HaveOccurred())

		btc, err = bigtable.NewClient(context.Background(),
			"fake-project", "fake-instance", option.WithGRPCConn(conn))
		Expect(err).NotTo(HaveOccurred())

		btac, err = bigtable.NewAdminClient(context.Background(),
			"fake-project", "fake-instance", option.WithGRPCConn(conn))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterSuite(func() {
		//Expect(btc.Close()).To(Succeed())
		srv.Close()
	})

	RunSpecs(t, "GcpBigtable Suite")
}
