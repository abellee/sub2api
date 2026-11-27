package service

import (
	"context"
	"fmt"
	"reflect"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"lotteryd/api/events"
)

type gameplayActorKey struct{}

func WithGameplayActor(ctx context.Context, actorID int64) context.Context {
	return context.WithValue(ctx, gameplayActorKey{}, actorID)
}

// Hooks snapshot primitive fields. Outer transactions publish only after a
// successful commit; hook failures never change the caller's result.
func publishGameplay(ctx context.Context, kind string, userID int64, reference string, fields ...any) {
	if !events.Enabled() {
		return
	}
	defer func() { _ = recover() }()
	data := make(map[string]string, len(fields)/2)
	if ctx != nil {
		if actor, ok := ctx.Value(gameplayActorKey{}).(int64); ok && actor > 0 {
			data["actor_id"] = gameplayValue(actor)
		}
	}
	for i := 0; i+1 < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			continue
		}
		data[key] = gameplayValue(fields[i+1])
	}
	if ctx != nil {
		if tx := dbent.TxFromContext(ctx); tx != nil {
			tx.OnCommit(func(next dbent.Committer) dbent.Committer {
				return dbent.CommitFunc(func(commitCtx context.Context, tx *dbent.Tx) error {
					err := next.Commit(commitCtx, tx)
					if err == nil {
						events.Publish(kind, userID, reference, data)
					}
					return err
				})
			})
			return
		}
	}
	events.Publish(kind, userID, reference, data)
}

func gameplayValue(value any) string {
	if value == nil {
		return ""
	}
	if instant, ok := value.(time.Time); ok {
		return instant.UTC().Format(time.RFC3339)
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return ""
		}
		return gameplayValue(rv.Elem().Interface())
	}
	return fmt.Sprint(value)
}

func gameplaySubscription(ctx context.Context, kind string, sub *UserSubscription, reference string) {
	if sub == nil {
		return
	}
	publishGameplay(ctx, kind, sub.UserID, reference, "subscription_id", sub.ID, "group_id", sub.GroupID, "status", sub.Status, "expires_at", sub.ExpiresAt)
}

func gameplayPayment(ctx context.Context, kind string, order *dbent.PaymentOrder) {
	if order == nil {
		return
	}
	status := order.Status
	switch kind {
	case "payment.balance_completed", "payment.subscription_completed":
		status = OrderStatusCompleted
	case "payment.refund_requested":
		status = OrderStatusRefundRequested
	case "payment.cancelled":
		status = OrderStatusCancelled
	case "payment.expired":
		status = OrderStatusExpired
	}
	publishGameplay(ctx, kind, order.UserID, order.OutTradeNo, "source_id", order.ID, "order_type", order.OrderType, "amount", order.Amount, "pay_amount", order.PayAmount, "currency", PaymentOrderCurrency(order), "status", status, "provider", order.ProviderKey)
}

// ObserveGameplayEvent is also used by repository operations that already know
// which rows changed, and by committed third-party identity bindings.
func ObserveGameplayEvent(ctx context.Context, kind string, userID int64, reference string, fields ...any) {
	publishGameplay(ctx, kind, userID, reference, fields...)
}
