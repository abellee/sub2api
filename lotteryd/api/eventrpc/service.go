package eventrpc

import (
	"context"
	"encoding/json"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"lotteryd/api/events"
)

const Method = "/lotteryd.v1.EventService/Emit"

type Receiver interface {
	ReceiveEvent(context.Context, events.Event) error
}

func Send(ctx context.Context, conn grpc.ClientConnInterface, e events.Event) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return conn.Invoke(ctx, Method, wrapperspb.Bytes(raw), &emptypb.Empty{})
}
func Register(s *grpc.Server, r Receiver) {
	s.RegisterService(&grpc.ServiceDesc{ServiceName: "lotteryd.v1.EventService", HandlerType: (*Receiver)(nil), Methods: []grpc.MethodDesc{{MethodName: "Emit", Handler: handle}}}, r)
}
func handle(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := &wrapperspb.BytesValue{}
	if err := dec(req); err != nil {
		return nil, err
	}
	call := func(ctx context.Context, value any) (any, error) {
		raw := value.(*wrapperspb.BytesValue).Value
		if len(raw) > 64<<10 {
			return nil, status.Error(codes.InvalidArgument, "event too large")
		}
		var e events.Event
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid event")
		}
		if err := srv.(Receiver).ReceiveEvent(ctx, e); err != nil {
			return nil, err
		}
		return &emptypb.Empty{}, nil
	}
	if interceptor == nil {
		return call(ctx, req)
	}
	return interceptor(ctx, req, &grpc.UnaryServerInfo{Server: srv, FullMethod: Method}, call)
}
