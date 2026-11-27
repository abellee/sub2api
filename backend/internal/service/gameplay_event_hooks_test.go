package service

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"lotteryd/api/events"
)

func TestGameplayHookPublishesOnlyAfterTransactionCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	queue := make(chan events.Event, 2)
	events.SetSink(func(e events.Event) { queue <- e })
	defer events.SetSink(nil)
	mock.ExpectBegin()
	tx, err := client.Tx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ObserveGameplayEvent(dbent.NewTxContext(context.Background(), tx), "payment.refunded", 2, "order-1", "amount", 3.5)
	select {
	case <-queue:
		t.Fatal("uncommitted reward event escaped transaction")
	default:
	}
	mock.ExpectCommit()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-queue:
		if e.UserID != 2 || e.Data["amount"] != "3.5" {
			t.Fatalf("wrong event snapshot: %#v", e)
		}
	default:
		t.Fatal("committed event missing")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGameplayHookRollbackAndSenderFailureDoNotAffectBusiness(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	count := 0
	events.SetSink(func(events.Event) { count++ })
	defer events.SetSink(nil)
	mock.ExpectBegin()
	tx, err := client.Tx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ObserveGameplayEvent(dbent.NewTxContext(context.Background(), tx), "subscription.assigned", 2, "sub-1")
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("rolled back business emitted an event")
	}
	events.SetSink(func(events.Event) { panic("receiver unavailable") })
	ObserveGameplayEvent(context.Background(), "user.password_changed", 2, "")
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
