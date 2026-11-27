package eventrpc_test

import (
	"context"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"lotteryd/api/eventrpc"
	"lotteryd/api/events"
)

type fakeEventReceiver struct {
	mu     sync.Mutex
	events []events.Event
	err    error
}

func (receiver *fakeEventReceiver) ReceiveEvent(_ context.Context, event events.Event) error {
	receiver.mu.Lock()
	defer receiver.mu.Unlock()
	receiver.events = append(receiver.events, event)
	return receiver.err
}

func eventRPCConnection(t *testing.T, receiver *fakeEventReceiver) *grpc.ClientConn {
	t.Helper()
	listener := bufconn.Listen(256 << 10)
	server := grpc.NewServer()
	eventrpc.Register(server, receiver)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	connection, err := grpc.NewClient("passthrough:///gameplay-events-test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return listener.DialContext(ctx)
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return connection
}

func TestEventRPCSendPreservesTrustedIdentityAndDecimalFields(t *testing.T) {
	receiver := &fakeEventReceiver{}
	connection := eventRPCConnection(t, receiver)
	event := events.Event{
		ID: "billing-event-2", Type: "billing.completed", Source: "sub2api", UserID: 2,
		Reference: "request-2", OccurredAt: time.Date(2026, 11, 28, 1, 2, 3, 456, time.UTC),
		Data: map[string]string{"actual_cost": "0.00000001", "total_cost": "12345678901234567890.12345678", "model": "gpt-6", "input_tokens": "12"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := eventrpc.Send(ctx, connection, event); err != nil {
		t.Fatalf("send event: %v", err)
	}
	receiver.mu.Lock()
	defer receiver.mu.Unlock()
	if len(receiver.events) != 1 || !reflect.DeepEqual(receiver.events[0], event) {
		t.Fatalf("event changed in gRPC transport: %+v", receiver.events)
	}
}

func TestEventRPCSendPreservesReceiverStatusError(t *testing.T) {
	receiver := &fakeEventReceiver{err: status.Error(codes.Unavailable, "receiver temporarily unavailable")}
	connection := eventRPCConnection(t, receiver)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := eventrpc.Send(ctx, connection, events.Event{ID: "receiver-failure", Type: "billing.completed", UserID: 2})
	if status.Code(err) != codes.Unavailable || status.Convert(err).Message() != "receiver temporarily unavailable" {
		t.Fatalf("receiver error not preserved: %v", err)
	}
}

func TestEventRPCRejectsPayloadOver64KiBBeforeCallingReceiver(t *testing.T) {
	receiver := &fakeEventReceiver{}
	connection := eventRPCConnection(t, receiver)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := eventrpc.Send(ctx, connection, events.Event{ID: "oversized-event", Type: "billing.completed", UserID: 2, Data: map[string]string{"oversized": strings.Repeat("x", 64<<10)}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("oversized event error = %v", err)
	}
	receiver.mu.Lock()
	defer receiver.mu.Unlock()
	if len(receiver.events) != 0 {
		t.Fatal("oversized payload reached receiver")
	}
}
