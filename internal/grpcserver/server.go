// Package grpcserver serves metric batches over gRPC.
package grpcserver

import (
	"context"
	"errors"
	"net"

	models "github.com/tomkqwe/metrics/internal/model"
	pb "github.com/tomkqwe/metrics/internal/proto"
	"github.com/tomkqwe/metrics/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type metricsServer struct {
	pb.UnimplementedMetricsServer
	service service.Service
}

// New registers the metrics service and its trusted-subnet interceptor.
func New(srv service.Service, subnet *net.IPNet) *grpc.Server {
	server := grpc.NewServer(grpc.UnaryInterceptor(trustedSubnet(subnet)))
	pb.RegisterMetricsServer(server, &metricsServer{service: srv})
	return server
}

func trustedSubnet(subnet *net.IPNet) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if subnet != nil {
			values := metadata.ValueFromIncomingContext(ctx, "x-real-ip")
			if len(values) != 1 || !subnet.Contains(net.ParseIP(values[0])) {
				return nil, status.Error(codes.PermissionDenied, "agent IP outside trusted subnet or missing")
			}
		}
		return next(ctx, req)
	}
}

func (s *metricsServer) UpdateMetrics(ctx context.Context, req *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	batch := make([]models.Metric, 0, len(req.GetMetrics()))
	for _, m := range req.GetMetrics() {
		if m == nil {
			return nil, status.Error(codes.InvalidArgument, "missing metric")
		}
		metric := models.Metric{ID: m.Id}
		switch m.Type {
		case pb.Metric_GAUGE:
			metric.MType, metric.Value = models.MetricTypeGauge, &m.Value
		case pb.Metric_COUNTER:
			metric.MType, metric.Delta = models.MetricTypeCounter, &m.Delta
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unknown metric type %d", m.Type)
		}
		batch = append(batch, metric)
	}
	if err := s.service.UpdateMetricsJSON(ctx, batch); err != nil {
		if errors.Is(err, service.ErrInvalidMetricName) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, status.FromContextError(err).Err()
		}
		return nil, status.Error(codes.Internal, "failed to save metrics")
	}
	return &pb.UpdateMetricsResponse{}, nil
}
