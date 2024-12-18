package serverapp

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type InterceptorTestSuite struct {
	suite.Suite
}

func (i *InterceptorTestSuite) TestInterceptors() {
	// -- Given
	//
	hc := &HealthCheck{}

	ctx := context.Background()
	unary, stream := DefaultServerInterceptors()
	cu, cs := ContextMutatorServerInterceptors(func(ctx context.Context) context.Context {
		return withLogger(ctx, slog.Default())
	})
	unary = append(unary, cu)
	stream = append(stream, cs)
	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(stream...),
	)
	grpc_health_v1.RegisterHealthServer(s, hc)
	list, _ := net.Listen("tcp", "localhost:0")
	go func() {
		err := s.Serve(list)
		i.NoError(err)
	}()
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := WaitUntilReady(dialCtx, list.Addr().String())
	if err != nil {
		i.FailNow(err.Error())
	}

	cl, err := NewClientFactory(DefaultDialOpts()...).NewClient(list.Addr().String())
	if err != nil {
		i.FailNow(err.Error())
	}

	h := grpc_health_v1.NewHealthClient(cl)

	// -- When
	//
	resp, err := h.Check(ctx, &grpc_health_v1.HealthCheckRequest{})

	// -- Then
	//
	if i.NoError(err) {
		i.Equal(grpc_health_v1.HealthCheckResponse_SERVING, resp.GetStatus())
	}
}

func TestInterceptorTestSuite(t *testing.T) {
	suite.Run(t, new(InterceptorTestSuite))
}

type loggerKey struct{}

func withLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, l)
}
