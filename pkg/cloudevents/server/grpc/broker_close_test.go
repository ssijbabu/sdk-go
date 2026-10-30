package grpc

import (
	"context"
	"testing"

	pbv1 "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc/protobuf/v1"
)

func TestCloseClusterCancelsOnlyThatCluster(t *testing.T) {
	broker := NewGRPCBroker(NewBrokerOptions())
	ctx := context.Background()

	cancelled := map[string]bool{}
	register := func(id, cluster string) {
		if err := broker.registerSubscriber(ctx, id, dataType, &pbv1.SubscriptionRequest{ClusterName: cluster},
			func() { cancelled[id] = true }, nil); err != nil {
			t.Fatal(err)
		}
	}
	register("a1", "cluster1")
	register("a2", "cluster1")
	register("b1", "cluster2")

	if n := broker.CloseCluster(ctx, "cluster1"); n != 2 {
		t.Fatalf("want 2 streams closed, got %d", n)
	}
	if !cancelled["a1"] || !cancelled["a2"] || cancelled["b1"] {
		t.Fatalf("unexpected cancellations %v", cancelled)
	}
	if n := broker.CloseCluster(ctx, "unknown"); n != 0 {
		t.Fatalf("want no streams closed for an unknown cluster, got %d", n)
	}
}
