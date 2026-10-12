// Package rpc 提供 lotteryd 的 gRPC 服务（积分/道具奖励）。
// 主站 sub2api 在充值回调完成后异步调用 NotifyPayment。
package rpc

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	rewardsv1 "lotteryd/api/gen/lotteryd/v1"
	"lotteryd/internal/store"
)

// MetaToken gRPC 鉴权 metadata 键。
const MetaToken = "x-internal-token"

// Server 实现 rewardsv1.RewardServiceServer。
type Server struct {
	rewardsv1.UnimplementedRewardServiceServer
	st    *store.Store
	token string
}

// NewServer 构造奖励服务实现；token 非空时校验 x-internal-token。
func NewServer(st *store.Store, token string) *Server {
	return &Server{st: st, token: token}
}

// NewGRPCServer 构造 gRPC server；token 非空时校验 x-internal-token。
func NewGRPCServer(st *store.Store, token string) *grpc.Server {
	impl := NewServer(st, token)
	s := grpc.NewServer(grpc.ChainUnaryInterceptor(impl.authInterceptor()))
	rewardsv1.RegisterRewardServiceServer(s, impl)
	return s
}

func (s *Server) authInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if s.token != "" {
			md, _ := metadata.FromIncomingContext(ctx)
			if join(md.Get(MetaToken)) != s.token {
				return nil, status.Error(codes.Unauthenticated, "invalid internal token")
			}
		}
		return handler(ctx, req)
	}
}

func join(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	return strings.TrimSpace(vals[0])
}

// NotifyPayment 充值回调通知：按配置发放积分，处理使用意图，全部幂等。
func (s *Server) NotifyPayment(ctx context.Context, req *rewardsv1.NotifyPaymentRequest) (*rewardsv1.NotifyPaymentResponse, error) {
	if req.GetOrderNo() == "" {
		return nil, status.Error(codes.InvalidArgument, "order_no is required")
	}
	if req.GetUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	resp := &rewardsv1.NotifyPaymentResponse{Accepted: true}
	ext := map[string]string{}
	for k, v := range req.GetMetadata() {
		ext[k] = v
	}

	// 1. 充值积分：按 sync_state 里的 payment_points_rate（每实付 1 单位货币
	//    送多少积分，可带小数）发放；0 或未配置表示不发放。PAID/COMPLETED 均发，
	//    幂等保证回调重放不会重复加分。
	if grant := s.rechargeGrant(req); grant > 0 {
		balance, granted, err := s.st.GrantPoints(req.GetUserId(), grant, "payment_recharge",
			"payment_order", req.GetOrderNo(), "充值发放", ext)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "grant points: %v", err)
		}
		eff := &rewardsv1.AppliedEffect{
			Kind:         "grant_points",
			Amount:       strconv.FormatInt(grant, 10),
			BalanceAfter: balance,
		}
		if !granted {
			eff.Error = "already granted"
		}
		resp.Effects = append(resp.Effects, eff)
	}

	// 2. 使用意图（预留）：消耗积分 / 使用道具。单项失败不回滚其余项，
	//    失败原因写进对应 effect 的 error 字段。
	for i, intent := range req.GetIntents() {
		eff := s.applyIntent(req, i, intent, ext)
		resp.Effects = append(resp.Effects, eff)
	}

	slog.Info("grpc notify payment", "order", req.GetOrderNo(), "user", req.GetUserId(),
		"phase", req.GetPhase(), "effects", len(resp.GetEffects()))
	return resp, nil
}

// rechargeGrant 计算充值应发放的积分数量（0 = 不发放）。
func (s *Server) rechargeGrant(req *rewardsv1.NotifyPaymentRequest) int64 {
	phase := strings.ToUpper(req.GetPhase())
	if phase != "PAID" && phase != "COMPLETED" {
		return 0
	}
	if req.GetOrderType() != "" && req.GetOrderType() != "balance" {
		return 0
	}
	rateRaw, err := s.st.GetState("payment_points_rate")
	if err != nil || strings.TrimSpace(rateRaw) == "" {
		return 0
	}
	rate, err := strconv.ParseFloat(strings.TrimSpace(rateRaw), 64)
	if err != nil || rate <= 0 {
		return 0
	}
	paid, err := strconv.ParseFloat(strings.TrimSpace(req.GetPayAmount()), 64)
	if err != nil || paid <= 0 {
		return 0
	}
	grant := int64(math.Floor(paid * rate))
	if grant <= 0 {
		return 0
	}
	return grant
}

// applyIntent 处理单个使用意图，永不 panic。
func (s *Server) applyIntent(req *rewardsv1.NotifyPaymentRequest, idx int, intent *rewardsv1.PaymentIntent, ext map[string]string) *rewardsv1.AppliedEffect {
	eff := &rewardsv1.AppliedEffect{Kind: intent.GetKind(), ItemCode: intent.GetItemCode()}
	defer func() {
		if r := recover(); r != nil {
			eff.Error = fmt.Sprintf("panic: %v", r)
		}
	}()
	refID := fmt.Sprintf("%s/intent/%d", req.GetOrderNo(), idx)
	switch intent.GetKind() {
	case "spend_points":
		amount, err := strconv.ParseInt(strings.TrimSpace(intent.GetAmount()), 10, 64)
		if err != nil || amount <= 0 {
			eff.Error = "invalid points amount"
			return eff
		}
		balance, err := s.st.SpendPoints(req.GetUserId(), amount, "recharge_intent",
			"payment_order", refID, "充值消耗积分", ext)
		if err != nil {
			eff.Error = err.Error()
			return eff
		}
		eff.Amount = strconv.FormatInt(amount, 10)
		eff.BalanceAfter = balance
		return eff
	case "use_item":
		item, err := s.st.GetItemByCode(intent.GetItemCode())
		if err != nil {
			eff.Error = fmt.Sprintf("item %q not found", intent.GetItemCode())
			return eff
		}
		qty := int(intent.GetQuantity())
		if qty <= 0 {
			qty = 1
		}
		if err := s.st.UseUserItem(req.GetUserId(), item.ID, qty,
			"payment_order", refID, "充值使用道具", ext); err != nil {
			eff.Error = err.Error()
			return eff
		}
		eff.Amount = strconv.Itoa(qty)
		return eff
	default:
		eff.Error = fmt.Sprintf("unknown intent kind %q", intent.GetKind())
		return eff
	}
}

// GetPointsBalance 查询用户积分余额。
func (s *Server) GetPointsBalance(ctx context.Context, req *rewardsv1.GetPointsBalanceRequest) (*rewardsv1.GetPointsBalanceResponse, error) {
	if req.GetUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	b, err := s.st.GetPointsBalance(req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get points: %v", err)
	}
	return &rewardsv1.GetPointsBalanceResponse{
		Balance:     b.Balance,
		TotalEarned: b.TotalEarned,
		TotalSpent:  b.TotalSpent,
	}, nil
}

// ServeGRPC 在 addr 上启动 gRPC 服务，ctx 结束时优雅退出。
func ServeGRPC(ctx context.Context, srv *grpc.Server, addr string) error {
	errCh := make(chan error, 1)
	go func() {
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			errCh <- fmt.Errorf("grpc listen %s: %w", addr, err)
			return
		}
		slog.Info("lotteryd grpc listening", "addr", addr)
		errCh <- srv.Serve(lis)
	}()
	select {
	case <-ctx.Done():
		stopped := make(chan struct{})
		go func() {
			srv.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			srv.Stop()
		}
		return nil
	case err := <-errCh:
		return err
	}
}
