package sender

import (
	"context"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/tomkqwe/metrics/internal/grpcserver"
	models "github.com/tomkqwe/metrics/internal/model"
	pb "github.com/tomkqwe/metrics/internal/proto"
	"github.com/tomkqwe/metrics/internal/repository/memstorage"
	"github.com/tomkqwe/metrics/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGRPCBatchDelivery(t *testing.T) {
	for _, restricted := range []bool{false, true} {
		var subnet *net.IPNet
		if restricted {
			_, subnet, _ = net.ParseCIDR("127.0.0.0/8")
		}
		srv, err := service.NewMetricService(memstorage.NewMemStorage())
		if err != nil {
			t.Fatal(err)
		}
		server := grpcserver.New(srv, subnet)
		listener := bufconn.Listen(1 << 20)
		go func() { _ = server.Serve(listener) }()
		t.Cleanup(server.Stop)
		conn, err := grpc.NewClient("passthrough:///metrics", grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		client := pb.NewMetricsClient(conn)
		s := GRPCSender{client: client, server: &url.URL{Host: "127.0.0.1:9090"}}
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		t.Cleanup(cancel)
		value, delta := 3.5, int64(4)
		batch := []models.Metric{{ID: "temperature", MType: models.MetricTypeGauge, Value: &value}, {ID: "polls", MType: models.MetricTypeCounter, Delta: &delta}}
		if err := s.Send(ctx, batch); err != nil {
			t.Fatal(err)
		}
		value = 0
		if err := s.Send(ctx, batch); err != nil {
			t.Fatal(err)
		}
		req := &pb.UpdateMetricsRequest{Metrics: []*pb.Metric{{Id: "polls", Type: pb.Metric_COUNTER, Delta: 1}}}
		for _, ips := range [][]string{nil, {"invalid"}, {"192.0.2.1"}, {"127.0.0.1", "127.0.0.2"}} {
			md := metadata.MD{}
			md.Set("x-real-ip", ips...)
			_, err := client.UpdateMetrics(metadata.NewOutgoingContext(ctx, md), req)
			want := codes.OK
			if restricted {
				want = codes.PermissionDenied
			}
			if status.Code(err) != want {
				t.Fatalf("subnet %v, IPs %v: got %v, want %v", subnet, ips, err, want)
			}
		}
		req.Metrics = append(req.Metrics, &pb.Metric{Id: "unknown", Type: 99})
		_, err = client.UpdateMetrics(metadata.AppendToOutgoingContext(ctx, "x-real-ip", "127.0.0.1"), req)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("invalid batch: %v", err)
		}
		wantCount := "12"
		if restricted {
			wantCount = "8"
		}
		if got, err := srv.GetMetricValue(ctx, "counter", "polls"); got != wantCount || err != nil {
			t.Fatalf("polls = %q, %v; want %s", got, err, wantCount)
		}
		if got, err := srv.GetMetricValue(ctx, "gauge", "temperature"); got != "0" || err != nil {
			t.Fatalf("temperature = %q, %v; want 0", got, err)
		}
	}
}
