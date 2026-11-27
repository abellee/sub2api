package store

import (
	"testing"

	"lotteryd/api/events"
)

func validUnitEventRule() EventRule {
	return EventRule{Name: "积分奖励", EventType: "billing.completed", Recipient: "user", Probability: 1000000, EveryN: 1, Action: EventAction{Kind: "grant_points", Points: 10}}
}

func TestEventRuleValidateProbabilityAndCooldown(t *testing.T) {
	for _, probability := range []int{0, 1, 250000, 999999, 1000000} {
		rule := validUnitEventRule()
		rule.Probability = probability
		if err := rule.Validate(); err != nil {
			t.Fatalf("probability %d: %v", probability, err)
		}
	}
	for _, probability := range []int{-1, 1000001} {
		rule := validUnitEventRule()
		rule.Probability = probability
		if rule.Validate() == nil {
			t.Fatalf("accepted invalid probability %d", probability)
		}
	}
	rule := validUnitEventRule()
	rule.CooldownSeconds = 315360000
	if err := rule.Validate(); err != nil {
		t.Fatalf("bounded cooldown: %v", err)
	}
	for _, cooldown := range []int64{-1, 315360001, 1000000000000} {
		rule.CooldownSeconds = cooldown
		if rule.Validate() == nil {
			t.Fatalf("accepted unsafe cooldown %d", cooldown)
		}
	}
}

func TestEventRuleValidateFiniteDecimalConditions(t *testing.T) {
	for _, value := range []string{"0", "-0.01", "0.00000001", "12345678901234567890.12345678"} {
		rule := validUnitEventRule()
		rule.Conditions = []EventCondition{{Field: "actual_cost", Operator: "gte", Value: value}}
		if err := rule.Validate(); err != nil {
			t.Fatalf("valid decimal %q: %v", value, err)
		}
	}
	for _, value := range []string{"", "NaN", "Infinity", "1/2", "1e100000", "0x100", "not a number"} {
		rule := validUnitEventRule()
		rule.Conditions = []EventCondition{{Field: "actual_cost", Operator: "gte", Value: value}}
		if rule.Validate() == nil {
			t.Fatalf("accepted invalid numeric condition %q", value)
		}
	}
}

func TestEventRuleValidateRejectsInvalidTargetsAndConditions(t *testing.T) {
	tests := map[string]func(*EventRule){
		"unknown event":     func(rule *EventRule) { rule.EventType = "does.not.exist" },
		"reserved event":    func(rule *EventRule) { rule.EventType = "shield.triggered" },
		"unknown recipient": func(rule *EventRule) { rule.Recipient = "somebody" },
		"empty name":        func(rule *EventRule) { rule.Name = " " },
		"unknown field": func(rule *EventRule) {
			rule.Conditions = []EventCondition{{Field: "api_key_secret", Operator: "eq", Value: "x"}}
		},
		"unknown operator": func(rule *EventRule) {
			rule.Conditions = []EventCondition{{Field: "model", Operator: "script", Value: "x"}}
		},
		"zero points":           func(rule *EventRule) { rule.Action.Points = 0 },
		"invalid item":          func(rule *EventRule) { rule.Action = EventAction{Kind: "grant_item", ItemID: 0, Quantity: 1} },
		"invalid item quantity": func(rule *EventRule) { rule.Action = EventAction{Kind: "grant_item", ItemID: 1, Quantity: 0} },
		"reversed time window":  func(rule *EventRule) { rule.StartsAt = "2026-11-29T00:00:00Z"; rule.EndsAt = "2026-11-28T00:00:00Z" },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			rule := validUnitEventRule()
			change(&rule)
			if rule.Validate() == nil {
				t.Fatal("accepted invalid rule")
			}
		})
	}
}

func TestEventConditionsFailClosedWhenFieldsAreMissing(t *testing.T) {
	event := events.Event{UserID: 2, Data: map[string]string{"model": "gpt-6"}}
	for _, operator := range []string{"eq", "ne", "in", "contains", "gt", "gte", "lt", "lte"} {
		rule := validUnitEventRule()
		rule.Conditions = []EventCondition{{Field: "actual_cost", Operator: operator, Value: "1"}}
		if matchesConditions(rule, event) {
			t.Fatalf("missing field matched operator %q", operator)
		}
	}
	rule := validUnitEventRule()
	rule.Conditions = []EventCondition{{Field: "user_id", Operator: "eq", Value: "2"}}
	if !matchesConditions(rule, event) {
		t.Fatal("user identity did not match the trusted event user")
	}
}

func TestEventConditionsPreserveDecimalPrecision(t *testing.T) {
	event := events.Event{UserID: 2, Data: map[string]string{"actual_cost": "12345678901234567890.00000001", "model": "gpt-6"}}
	tests := []struct {
		operator, value string
		match           bool
	}{
		{"gt", "12345678901234567890.00000000", true},
		{"lt", "12345678901234567890.00000002", true},
		{"gte", "12345678901234567890.00000001", true},
		{"lte", "12345678901234567890.00000001", true},
		{"gt", "12345678901234567890.00000001", false},
		{"lt", "12345678901234567890.00000001", false},
	}
	for _, test := range tests {
		rule := validUnitEventRule()
		rule.Conditions = []EventCondition{{Field: "actual_cost", Operator: test.operator, Value: test.value}}
		if got := matchesConditions(rule, event); got != test.match {
			t.Errorf("%s %s: got %v, want %v", test.operator, test.value, got, test.match)
		}
	}
	rule := validUnitEventRule()
	rule.Conditions = []EventCondition{{Field: "model", Operator: "in", Value: "gpt-5, gpt-6"}, {Field: "actual_cost", Operator: "gt", Value: "0"}}
	if !matchesConditions(rule, event) {
		t.Fatal("all matching conditions were rejected")
	}
	event.Data["actual_cost"] = "not numeric"
	if matchesConditions(rule, event) {
		t.Fatal("invalid numeric payload matched")
	}
}

func TestEventProbabilityIsDeterministicAndHonorsBoundaries(t *testing.T) {
	rule := validUnitEventRule()
	rule.ID = 12
	rule.Version = 3
	rule.Probability = 345678
	first := eventHit("fixed-test-secret", "stable-event-id", rule)
	for i := 0; i < 100; i++ {
		if eventHit("fixed-test-secret", "stable-event-id", rule) != first {
			t.Fatal("retry changed probability decision")
		}
	}
	for _, id := range []string{"event-a", "event-b", "event-c", "event-d"} {
		rule.Probability = 0
		if eventHit("fixed-test-secret", id, rule) {
			t.Fatal("zero probability hit")
		}
		rule.Probability = 1000000
		if !eventHit("fixed-test-secret", id, rule) {
			t.Fatal("certain probability missed")
		}
	}
}
