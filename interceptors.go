package serverapp

import (
	"context"
	"log/slog"
	"sync"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DefaultHistoBuckets default histogram buckets in seconds.
var DefaultHistoBuckets = []float64{0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120}

// DefaultServerInterceptors is a convenience func for default server interceptors.
// Should be placed at the end of the chain.
func DefaultServerInterceptors() ([]grpc.UnaryServerInterceptor, []grpc.StreamServerInterceptor) {
	panicCounter := NewPanicCounterMetric()
	logU, logS := LoggingServerInterceptors(SlogLogger(slog.Default()))
	recU, recS := RecoveryServerInterceptors(panicCounter)
	metU, metS := MetricsServerInterceptors(prometheus.DefaultRegisterer)

	return []grpc.UnaryServerInterceptor{logU, metU, recU}, []grpc.StreamServerInterceptor{logS, metS, recS}
}

// LoggingServerInterceptors provide interceptors for logging the start and end of a call.
func LoggingServerInterceptors(logger logging.Logger) (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	opts := []logging.Option{
		logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
		logging.WithLevels(func(code codes.Code) logging.Level {
			if code == codes.OK || code == codes.DeadlineExceeded {
				return logging.LevelDebug
			}
			return logging.LevelError
		}),
	}

	unary := logging.UnaryServerInterceptor(logger, opts...)

	stream := logging.StreamServerInterceptor(logger, opts...)

	return unary, stream
}

// RecoveryServerInterceptors handles recovery in the case of a panic.
func RecoveryServerInterceptors(panicCounter prometheus.Counter) (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	rec := newRecoveryHandler(panicCounter)
	unary := recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(rec))

	stream := recovery.StreamServerInterceptor(recovery.WithRecoveryHandler(rec))

	return unary, stream
}

var registerServerMetOnce = sync.Once{}

// MetricsServerInterceptors generates metrics for every request received.
func MetricsServerInterceptors(reg prometheus.Registerer) (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	srvMetrics := grpcprom.NewServerMetrics(
		grpcprom.WithServerHandlingTimeHistogram(
			grpcprom.WithHistogramBuckets(DefaultHistoBuckets),
		),
	)

	registerServerMetOnce.Do(func() {
		reg.MustRegister(srvMetrics)
	})

	return srvMetrics.UnaryServerInterceptor(), srvMetrics.StreamServerInterceptor()
}

// NewPanicCounterMetric counter for panics during gRPC requests.
func NewPanicCounterMetric() prometheus.Counter {
	return promauto.NewCounter(prometheus.CounterOpts{
		Name: "grpc_req_panics_recovered_total",
		Help: "Total number of gRPC requests recovered from internal panic.",
	})
}

// ContextMutator mutates a context for a gRPC request.
type ContextMutator func(ctx context.Context) context.Context

// ContextMutatorServerInterceptors allows for context mutation e.g. to inject values into a context
// for every request.
func ContextMutatorServerInterceptors(mut ContextMutator) (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	unary := grpc.UnaryServerInterceptor(func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		ctx = mut(ctx)
		return handler(ctx, req)
	})

	stream := grpc.StreamServerInterceptor(func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := mut(ss.Context())
		return handler(srv, &serverStreamContext{ServerStream: ss, Ctx: ctx})
	})

	return unary, stream
}

type serverStreamContext struct {
	grpc.ServerStream
	Ctx context.Context
}

func (s *serverStreamContext) Context() context.Context {
	return s.Ctx
}

func newRecoveryHandler(counter prometheus.Counter) recovery.RecoveryHandlerFunc {
	return func(p any) (err error) {
		counter.Inc()
		return status.Errorf(codes.Internal, "%v", p)
	}
}
