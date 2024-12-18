package serverapp

import (
	"context"
	"log/slog"
	"sync"
	"time"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// DefaultClientInterceptors instantiate the default interceptors for a client.
func DefaultClientInterceptors() ([]grpc.UnaryClientInterceptor, []grpc.StreamClientInterceptor) {
	logU, logS := LoggingClientInterceptors(SlogLogger(slog.Default()))
	metU, metS := MetricsClientInterceptors(prometheus.DefaultRegisterer)

	unary := []grpc.UnaryClientInterceptor{
		logU,
		metU,
	}

	stream := []grpc.StreamClientInterceptor{
		logS,
		metS,
	}

	return unary, stream
}

// LoggingClientInterceptors instantiate the logging client-side interceptors.
func LoggingClientInterceptors(logger logging.Logger) (grpc.UnaryClientInterceptor, grpc.StreamClientInterceptor) {
	opts := []logging.Option{
		logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
		logging.WithLevels(func(code codes.Code) logging.Level {
			if code == codes.OK || code == codes.DeadlineExceeded {
				return logging.LevelDebug
			}
			return logging.LevelError
		}),
	}

	unary := logging.UnaryClientInterceptor(logger, opts...)

	stream := logging.StreamClientInterceptor(logger, opts...)

	return unary, stream
}

var registerClientMetOnce = sync.Once{}

// MetricsClientInterceptors instantiate the client metrics interceptors.
func MetricsClientInterceptors(reg prometheus.Registerer) (grpc.UnaryClientInterceptor, grpc.StreamClientInterceptor) {
	clMetrics := grpcprom.NewClientMetrics(
		grpcprom.WithClientHandlingTimeHistogram(
			grpcprom.WithHistogramBuckets(DefaultHistoBuckets),
		),
	)

	registerClientMetOnce.Do(func() {
		reg.MustRegister(clMetrics)
	})

	return clMetrics.UnaryClientInterceptor(), clMetrics.StreamClientInterceptor()
}

// WaitUntilReady waits until the gRPC service is ready by calling IsReady.
// This should be used with a timeout via context.WithTimeout.
func WaitUntilReady(ctx context.Context, addr string) error {
	ticker := time.NewTicker(1 * time.Second)
	var err error
	var done bool

	for !done {
		done, err = func() (bool, error) {
			toCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()
			if IsReady(toCtx, addr) {
				return true, nil
			}
			select {
			case <-ctx.Done():
				return false, context.Cause(ctx)
			case <-ticker.C:
			}
			return false, nil
		}()
		if err != nil {
			return err
		}
	}
	return err
}

// ClientFactory creates GRPC connections.
type ClientFactory interface {
	NewClient(addr string, opts ...grpc.DialOption) (ClientConn, error)
}

// ClientConn a wrapper interface to represent a GRPC connection.
type ClientConn interface {
	grpc.ClientConnInterface
	Close() error
}

// NewClientFactory constructs a new ClientFactory.
func NewClientFactory(o ...grpc.DialOption) ClientFactory {
	out := &clientFactory{
		DialOpts: o,
	}

	return out
}

type clientFactory struct {
	DialOpts []grpc.DialOption
}

func (c *clientFactory) NewClient(addr string, opts ...grpc.DialOption) (ClientConn, error) {
	return grpc.NewClient(addr, append(opts, c.DialOpts...)...)
}
