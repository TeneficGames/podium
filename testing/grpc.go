package testing

import (
	"github.com/TeneficGames/podium/api"
	pb "github.com/TeneficGames/podium/proto/podium/api/v1"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// SetupGRPC sets up the environment for grpc communication, starting the app and creating a connected client
func SetupGRPC(app *api.App, f func(pb.PodiumServiceClient)) {
	InitializeTestServer(app)

	conn, err := grpc.NewClient(app.GRPCEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	Expect(err).NotTo(HaveOccurred())
	defer func() {
		_ = conn.Close()
	}()

	cli := pb.NewPodiumServiceClient(conn)

	f(cli)
}
