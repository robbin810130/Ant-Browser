package release

import "testing"

func TestRepairPlanForOutdatedResource(t *testing.T) {
	plan := BuildRepairPlan(CheckResult{
		State: StateRepairable,
		Items: []FailureItem{{
			Code:       "PKG-RESOURCE-OUTDATED",
			Repairable: true,
		}},
	})
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != "fetch-package" {
		t.Fatalf("unexpected repair plan: %#v", plan)
	}
}

func TestRepairPlanForMissingPointer(t *testing.T) {
	plan := BuildRepairPlan(CheckResult{
		State: StateRepairable,
		Items: []FailureItem{{
			Code:       "ENV-RUNTIME-POINTER-MISSING",
			Repairable: true,
		}},
	})
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != "rewrite-active-pointer" {
		t.Fatalf("unexpected repair plan: %#v", plan)
	}
}

func TestRepairPlanRejectsBlockedItems(t *testing.T) {
	_, err := ExecuteRepair(nil, CheckResult{
		State: StateBlocked,
		Items: []FailureItem{{
			Code:       "NET-PROXY-AUTH-FAILED",
			Repairable: false,
		}},
	})
	if err == nil {
		t.Fatal("expected blocked result to reject auto repair")
	}
}
