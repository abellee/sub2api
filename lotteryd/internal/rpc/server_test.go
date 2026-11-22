package rpc_test

import (
	"context"
	"testing"

	rewardsv1 "lotteryd/api/gen/lotteryd/v1"
	"lotteryd/internal/rpc"
	"lotteryd/internal/store"
	"lotteryd/internal/store/storetest"
)

func newServer(t *testing.T) (*rpc.Server, *store.Store) {
	t.Helper()
	st := storetest.Open(t)
	return rpc.NewServer(st, ""), st
}

func TestNotifyPaymentGrant(t *testing.T) {
	srv, st := newServer(t)
	ctx := context.Background()

	st.SetState("payment_points_rate", "10")

	resp, err := srv.NotifyPayment(ctx, &rewardsv1.NotifyPaymentRequest{
		OrderNo: "sub2_100", UserId: 1, Phase: "COMPLETED",
		OrderType: "balance", PayAmount: "12.5",
		Metadata: map[string]string{"provider": "easypay"},
	})
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	if !resp.GetAccepted() || len(resp.GetEffects()) != 1 {
		t.Fatalf("resp = %+v", resp)
	}
	eff := resp.GetEffects()[0]
	if eff.GetKind() != "grant_points" || eff.GetAmount() != "125" || eff.GetBalanceAfter() != 125 {
		t.Fatalf("effect = %+v", eff)
	}
	// 重放：幂等，不再新增效果差异。
	resp2, err := srv.NotifyPayment(ctx, &rewardsv1.NotifyPaymentRequest{
		OrderNo: "sub2_100", UserId: 1, Phase: "COMPLETED", OrderType: "balance", PayAmount: "12.5",
	})
	if err != nil {
		t.Fatalf("notify replay: %v", err)
	}
	if resp2.GetEffects()[0].GetError() == "" {
		t.Fatalf("replay should report already granted: %+v", resp2)
	}
	if b, _ := st.GetPointsBalance(1); b.Balance != 125 {
		t.Fatalf("balance after replay = %d, want 125", b.Balance)
	}
}

func TestNotifyPaymentIntents(t *testing.T) {
	srv, st := newServer(t)
	ctx := context.Background()

	if _, err := st.UpsertItem("gift", "新手礼", "", "", "{}"); err != nil {
		t.Fatalf("upsert item: %v", err)
	}
	if err := st.AddUserItems(2, mustItemID(t, st, "gift"), 1); err != nil {
		t.Fatalf("add item: %v", err)
	}

	resp, err := srv.NotifyPayment(ctx, &rewardsv1.NotifyPaymentRequest{
		OrderNo: "sub2_200", UserId: 2, Phase: "PAID",
		Intents: []*rewardsv1.PaymentIntent{
			{Kind: "use_item", ItemCode: "gift", Quantity: 1},
			{Kind: "use_item", ItemCode: "gift", Quantity: 5},
			{Kind: "spend_points", Amount: "10"},
			{Kind: "spend_points", Amount: "abc"},
		},
	})
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(resp.GetEffects()) != 4 {
		t.Fatalf("effects = %+v", resp.GetEffects())
	}
	if resp.GetEffects()[0].GetError() != "" {
		t.Fatalf("first use should succeed: %+v", resp.GetEffects()[0])
	}
	if resp.GetEffects()[1].GetError() == "" {
		t.Fatalf("second use should fail (insufficient): %+v", resp.GetEffects()[1])
	}
	// spend_points：无余额记录 → 余额不足报错。
	if resp.GetEffects()[2].GetError() == "" {
		t.Fatalf("spend without balance should fail: %+v", resp.GetEffects()[2])
	}
	if resp.GetEffects()[3].GetError() == "" {
		t.Fatalf("invalid amount should fail: %+v", resp.GetEffects()[3])
	}
}

func TestNotifyPaymentValidation(t *testing.T) {
	srv, _ := newServer(t)
	ctx := context.Background()
	if _, err := srv.NotifyPayment(ctx, &rewardsv1.NotifyPaymentRequest{}); err == nil {
		t.Fatal("empty request should fail")
	}
}

func mustItemID(t *testing.T, st *store.Store, code string) int64 {
	t.Helper()
	it, err := st.GetItemByCode(code)
	if err != nil {
		t.Fatalf("item %s: %v", code, err)
	}
	return it.ID
}
