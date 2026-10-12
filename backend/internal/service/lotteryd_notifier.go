package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/Wei-Shaw/sub2api/internal/config"
	rewardsv1 "lotteryd/api/gen/lotteryd/v1"
)

// LotterydNotifier 充值回调后向 lotteryd 发送异步通知的客户端。
// 通知在独立协程中执行，任何失败只记日志，绝不影响充值主流程。
type LotterydNotifier struct {
	addr    string
	token   string
	timeout time.Duration

	mu   sync.Mutex
	conn *grpc.ClientConn
}

// NewLotterydNotifier 根据配置构造通知器；grpc_addr 为空时返回 nil（未启用）。
func NewLotterydNotifier(cfg config.LotterydConfig) *LotterydNotifier {
	addr := strings.TrimSpace(cfg.GrpcAddr)
	if addr == "" {
		return nil
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &LotterydNotifier{addr: addr, token: strings.TrimSpace(cfg.Token), timeout: timeout}
}

func (n *LotterydNotifier) client() (rewardsv1.RewardServiceClient, func(), error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.conn != nil {
		return rewardsv1.NewRewardServiceClient(n.conn), func() {}, nil
	}
	conn, err := grpc.NewClient(n.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("grpc dial %s: %w", n.addr, err)
	}
	n.conn = conn
	return rewardsv1.NewRewardServiceClient(conn), func() {}, nil
}

// PaymentEvent 充值事件的业务字段（与渠道无关的规范化快照）。
type PaymentEvent struct {
	OrderNo     string
	TradeNo     string
	UserID      int64
	UserEmail   string
	OrderType   string
	ProviderKey string
	Amount      string
	PayAmount   string
	Currency    string
	Phase       string // PAID / COMPLETED / REFUNDED
	Metadata    map[string]string
}

// NotifyPaymentAsync 异步通知：内部起协程，立即返回，不阻塞、不传播错误。
func (n *LotterydNotifier) NotifyPaymentAsync(ev PaymentEvent) {
	if n == nil {
		return
	}
	go n.notifyPayment(ev)
}

func (n *LotterydNotifier) notifyPayment(ev PaymentEvent) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("lotteryd notify panic", "order", ev.OrderNo, "panic", r)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
	defer cancel()

	client, cleanup, err := n.client()
	if err != nil {
		slog.Warn("lotteryd notify skipped", "order", ev.OrderNo, "err", err)
		return
	}
	defer cleanup()

	if n.token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-internal-token", n.token)
	}

	req := &rewardsv1.NotifyPaymentRequest{
		OrderNo:     ev.OrderNo,
		TradeNo:     ev.TradeNo,
		UserId:      ev.UserID,
		UserEmail:   ev.UserEmail,
		OrderType:   ev.OrderType,
		ProviderKey: ev.ProviderKey,
		Amount:      ev.Amount,
		PayAmount:   ev.PayAmount,
		Currency:    ev.Currency,
		Phase:       ev.Phase,
		Metadata:    ev.Metadata,
	}
	resp, err := client.NotifyPayment(ctx, req)
	if err != nil {
		slog.Warn("lotteryd notify failed", "order", ev.OrderNo, "user", ev.UserID, "err", err)
		return
	}
	slog.Info("lotteryd notify done", "order", ev.OrderNo, "accepted", resp.GetAccepted(), "effects", len(resp.GetEffects()))
}

// NotifyPaymentCompleted 把一笔已完成的余额充值订单转成事件并异步通知。
// amount/payAmount 为订单上的 decimal 值（ent 映射为 float64）。
func (n *LotterydNotifier) NotifyPaymentCompleted(orderNo, tradeNo string, userID int64, userEmail, orderType, providerKey string, amount, payAmount float64, metadata map[string]string) {
	if n == nil || orderNo == "" || userID <= 0 {
		return
	}
	n.NotifyPaymentAsync(PaymentEvent{
		OrderNo:     orderNo,
		TradeNo:     tradeNo,
		UserID:      userID,
		UserEmail:   userEmail,
		OrderType:   orderType,
		ProviderKey: providerKey,
		Amount:      strconv.FormatFloat(amount, 'f', 2, 64),
		PayAmount:   strconv.FormatFloat(payAmount, 'f', 2, 64),
		Currency:    "CNY",
		Phase:       "COMPLETED",
		Metadata:    metadata,
	})
}
