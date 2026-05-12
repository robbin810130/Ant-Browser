package release

import (
	"context"
	"fmt"
)

type RepairAction struct {
	Kind      string `json:"kind"`
	PackageID string `json:"packageId,omitempty"`
}

type RepairPlan struct {
	Actions []RepairAction `json:"actions"`
}

type RepairExecutor interface {
	ApplyRepairAction(context.Context, RepairAction) error
	RunStartupCheck(context.Context) (CheckResult, error)
}

func BuildRepairPlan(result CheckResult) RepairPlan {
	var plan RepairPlan
	for _, item := range result.Items {
		switch item.Code {
		case "PKG-RESOURCE-OUTDATED", "PKG-RUNTIME-MISSING":
			plan.Actions = append(plan.Actions, RepairAction{
				Kind:      "fetch-package",
				PackageID: item.Code,
			})
		case "ENV-RUNTIME-POINTER-MISSING", "ENV-RUNTIME-POINTER-INVALID":
			plan.Actions = append(plan.Actions, RepairAction{Kind: "rewrite-active-pointer"})
		case "ENV-TEMP-LEFTOVER":
			plan.Actions = append(plan.Actions, RepairAction{Kind: "cleanup-temp"})
		}
	}
	return plan
}

func ExecuteRepair(manager RepairExecutor, result CheckResult) (CheckResult, error) {
	return ExecuteRepairWithContext(context.Background(), manager, result)
}

func ExecuteRepairWithContext(ctx context.Context, manager RepairExecutor, result CheckResult) (CheckResult, error) {
	if result.State == StateBlocked {
		return result, fmt.Errorf("blocked failures must not enter auto repair")
	}

	plan := BuildRepairPlan(result)
	if len(plan.Actions) == 0 {
		return result, nil
	}
	if manager == nil {
		return CheckResult{}, fmt.Errorf("repair executor is required")
	}

	for _, action := range plan.Actions {
		if err := manager.ApplyRepairAction(ctx, action); err != nil {
			return CheckResult{}, err
		}
	}
	return manager.RunStartupCheck(ctx)
}
