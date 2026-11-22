package store_test

import (
	"errors"
	"testing"

	"lotteryd/internal/store"
	"lotteryd/internal/store/storetest"
)

func TestGrantPointsIdempotent(t *testing.T) {
	st := storetest.Open(t)

	b, granted, err := st.GrantPoints(101, 50, "payment_recharge", "payment_order", "order-1", "", nil)
	if err != nil || !granted {
		t.Fatalf("first grant: granted=%v err=%v", granted, err)
	}
	if b != 50 {
		t.Fatalf("balance after grant = %d, want 50", b)
	}
	// 同一订单重复通知：不重复发放。
	b, granted, err = st.GrantPoints(101, 50, "payment_recharge", "payment_order", "order-1", "", nil)
	if err != nil || granted {
		t.Fatalf("replay grant: granted=%v err=%v", granted, err)
	}
	if b != 50 {
		t.Fatalf("balance after replay = %d, want 50", b)
	}
	// 不同订单可再次发放。
	b, granted, err = st.GrantPoints(101, 30, "payment_recharge", "payment_order", "order-2", "", nil)
	if err != nil || !granted || b != 80 {
		t.Fatalf("second order grant: granted=%v balance=%d err=%v", granted, b, err)
	}
}

func TestSpendPoints(t *testing.T) {
	st := storetest.Open(t)

	if _, _, err := st.GrantPoints(201, 100, "admin_adjust", "", "", "", nil); err != nil {
		t.Fatalf("grant: %v", err)
	}
	b, err := st.SpendPoints(201, 60, "recharge_intent", "payment_order", "order-9", "", nil)
	if err != nil || b != 40 {
		t.Fatalf("spend: balance=%d err=%v", b, err)
	}
	if _, err := st.SpendPoints(201, 41, "recharge_intent", "payment_order", "order-10", "", nil); !errors.Is(err, store.ErrInsufficientPoints) {
		t.Fatalf("over-spend err = %v, want ErrInsufficientPoints", err)
	}
	// 余额未被部分扣减。
	got, err := st.GetPointsBalance(201)
	if err != nil || got.Balance != 40 {
		t.Fatalf("balance after failed spend = %+v err=%v", got, err)
	}
	// 同 ref 的消耗幂等。
	if _, err := st.SpendPoints(201, 60, "recharge_intent", "payment_order", "order-9", "", nil); err != nil {
		t.Fatalf("replay spend: %v", err)
	}
	if got, _ := st.GetPointsBalance(201); got.Balance != 40 {
		t.Fatalf("balance after replay spend = %d, want 40", got.Balance)
	}
}

func TestItems(t *testing.T) {
	st := storetest.Open(t)

	itemID, err := st.UpsertItem("boost-card", "加成卡", "抽奖权重加成", "lottery_boost", `{"bonus":25}`)
	if err != nil {
		t.Fatalf("upsert item: %v", err)
	}
	it, err := st.GetItemByCode("boost-card")
	if err != nil || it.ID != itemID {
		t.Fatalf("get item: %+v err=%v", it, err)
	}
	if err := st.UseUserItem(301, itemID, 1, "payment_order", "o1", "", nil); !errors.Is(err, store.ErrInsufficientItem) {
		t.Fatalf("use before own err = %v", err)
	}
	if err := st.AddUserItems(301, itemID, 2); err != nil {
		t.Fatalf("add items: %v", err)
	}
	if err := st.AddUserItems(301, itemID, 1); err != nil {
		t.Fatalf("add items again: %v", err)
	}
	if err := st.UseUserItem(301, itemID, 4, "payment_order", "o2", "", nil); !errors.Is(err, store.ErrInsufficientItem) {
		t.Fatalf("use too many err = %v", err)
	}
	if err := st.UseUserItem(301, itemID, 3, "payment_order", "o2", "", nil); err != nil {
		t.Fatalf("use item: %v", err)
	}
	// 余额不足的消耗不产生使用记录：再消耗 1 张应失败且不影响状态。
	if err := st.UseUserItem(301, itemID, 1, "payment_order", "o3", "", nil); !errors.Is(err, store.ErrInsufficientItem) {
		t.Fatalf("use after empty err = %v", err)
	}
}
