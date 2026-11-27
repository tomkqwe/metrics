package sender

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"time"

	models "github.com/tomkqwe/metrics/internal/model"
	pb "github.com/tomkqwe/metrics/internal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// GRPCSender reuses a connection for concurrent batch reports.
type GRPCSender struct {
	conn   *grpc.ClientConn
	client pb.MetricsClient
	server *url.URL
}

func NewGRPCSender(address string) (*GRPCSender, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" || port == "" {
		return nil, fmt.Errorf("expected gRPC server host:port, got %q", address)
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &GRPCSender{conn: conn, client: pb.NewMetricsClient(conn), server: &url.URL{Host: address}}, nil
}

func (s *GRPCSender) Close() error { return s.conn.Close() }

func (s *GRPCSender) Send(ctx context.Context, metrics []models.Metric) error {
	if len(metrics) == 0 {
		return nil
	}
	req := &pb.UpdateMetricsRequest{Metrics: make([]*pb.Metric, 0, len(metrics))}
	for _, m := range metrics {
		if err := validateMetric(m); err != nil {
			return err
		}
		metric := &pb.Metric{Id: m.ID}
		if m.MType == models.MetricTypeCounter {
			metric.Type, metric.Delta = pb.Metric_COUNTER, *m.Delta
		} else {
			metric.Type, metric.Value = pb.Metric_GAUGE, *m.Value
		}
		req.Metrics = append(req.Metrics, metric)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ip, err := localIPForServer(ctx, s.server)
	if err != nil {
		return fmt.Errorf("determine agent IP: %w", err)
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "x-real-ip", ip.String())
	_, err = s.client.UpdateMetrics(ctx, req)
	return err
}
