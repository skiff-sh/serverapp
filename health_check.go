package serverapp

import (
	"context"

	"google.golang.org/grpc/health/grpc_health_v1"
)

var _ grpc_health_v1.HealthServer = &HealthCheck{}

// HealthCheck basic controller to handle health checks.
type HealthCheck struct{}

// Check unary method for health check.
func (h *HealthCheck) Check(_ context.Context, _ *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	resp := &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}
	return resp, nil
}

// Watch stream method for health check
func (h *HealthCheck) Watch(_ *grpc_health_v1.HealthCheckRequest, server grpc_health_v1.Health_WatchServer) error {
	resp := &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}

	return server.Send(resp)
}
