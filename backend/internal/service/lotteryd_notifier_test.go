package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestNewLotterydNotifierDisabledWhenAddrEmpty(t *testing.T) {
	if n := NewLotterydNotifier(config.LotterydConfig{}); n != nil {
		t.Fatalf("empty addr should disable notifier, got %+v", n)
	}
	if n := NewLotterydNotifier(config.LotterydConfig{GrpcAddr: "   "}); n != nil {
		t.Fatalf("blank addr should disable notifier, got %+v", n)
	}
}

func TestNotifyPaymentAsyncNilSafe(t *testing.T) {
	var n *LotterydNotifier
	n.NotifyPaymentAsync(PaymentEvent{OrderNo: "sub2_1", UserID: 1})
	n.NotifyPaymentCompleted("sub2_1", "", 1, "", "balance", "easypay", 10, 10, nil)
}

func TestNewLotterydNotifierTimeoutDefault(t *testing.T) {
	n := NewLotterydNotifier(config.LotterydConfig{GrpcAddr: "127.0.0.1:18101"})
	if n == nil {
		t.Fatal("expected notifier")
	}
	if n.timeout <= 0 {
		t.Fatalf("timeout = %s", n.timeout)
	}
}
