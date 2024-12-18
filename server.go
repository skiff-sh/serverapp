package serverapp

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// New opinionated constructor for a grpc.Server.
func New(unaryInts []grpc.UnaryServerInterceptor, streamInts []grpc.StreamServerInterceptor, opt ...grpc.ServerOption) *grpc.Server {
	opt = append(opt,
		grpc.ChainUnaryInterceptor(unaryInts...),
		grpc.ChainStreamInterceptor(streamInts...),
	)

	return grpc.NewServer(opt...)
}

// IsReady uses the health check API to see if a gRPC service is ready.
func IsReady(ctx context.Context, addr string) bool {
	dialer, err := grpc.NewClient(addr, DefaultDialOpts()...)
	if err != nil {
		return false
	}

	health := grpc_health_v1.NewHealthClient(dialer)
	resp, err := health.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return false
	}

	return resp.GetStatus() == grpc_health_v1.HealthCheckResponse_SERVING
}

// DefaultDialOpts returns grpc.DialOption that should be applied to all
// clients.
func DefaultDialOpts(o ...grpc.DialOption) []grpc.DialOption {
	u, s := DefaultClientInterceptors()

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainStreamInterceptor(s...),
		grpc.WithChainUnaryInterceptor(u...),
	}

	opts = append(opts, o...)

	return opts
}
