package workspace

import (
	"fmt"
	"sort"
	"strings"
)

func BuildOperationTasks(shops []ShopInstanceProjection, evidence RunEvidenceIndex, query OperationTaskQuery) OperationTasksPayload {
	tasks := make([]OperationTaskRecord, 0, len(shops))
	for _, shop := range shops {
		task := operationTaskForShop(shop, evidence.ByShop[strings.TrimSpace(shop.ShopID)])
		if !matchesOperationTaskQuery(task, query) {
			continue
		}
		tasks = append(tasks, task)
	}

	sort.SliceStable(tasks, func(i, j int) bool {
		leftRank := operationTaskStatusRank(tasks[i].Status)
		rightRank := operationTaskStatusRank(tasks[j].Status)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if tasks[i].UpdatedAt != tasks[j].UpdatedAt {
			return tasks[i].UpdatedAt > tasks[j].UpdatedAt
		}
		return tasks[i].ShopName < tasks[j].ShopName
	})

	total := len(tasks)
	if query.Limit > 0 && len(tasks) > query.Limit {
		tasks = tasks[:query.Limit]
	}
	return OperationTasksPayload{Items: tasks, Total: total}
}

func operationTaskForShop(shop ShopInstanceProjection, evidence ShopRunEvidence) OperationTaskRecord {
	shopID := strings.TrimSpace(shop.ShopID)
	shopName := strings.TrimSpace(shop.ShopName)
	if shopName == "" {
		shopName = shopID
	}

	if evidence.ActiveRun != nil {
		return OperationTaskRecord{
			TaskID:    buildOperationTaskID(shopID, evidence.ActiveRun.TaskType),
			ShopID:    shopID,
			ShopName:  shopName,
			TaskType:  operationTaskTypeForRun(evidence.ActiveRun),
			Title:     fmt.Sprintf("跟进 %s 当前执行", shopName),
			Status:    "running",
			UpdatedAt: firstNonEmpty(evidence.ActiveRun.StartedAt, evidence.ActiveRun.FinishedAt),
			RunID:     evidence.ActiveRun.RunID,
		}
	}

	if evidence.LatestFailure != nil {
		return OperationTaskRecord{
			TaskID:         buildOperationTaskID(shopID, evidence.LatestFailure.TaskType),
			ShopID:         shopID,
			ShopName:       shopName,
			TaskType:       operationTaskTypeForRun(evidence.LatestFailure),
			Title:          fmt.Sprintf("修复 %s 的执行失败", shopName),
			Status:         "failed",
			FailureMessage: strings.TrimSpace(evidence.LatestFailure.FailureMessage),
			UpdatedAt:      firstNonEmpty(evidence.LatestFailure.FinishedAt, evidence.LatestFailure.StartedAt),
			RunID:          evidence.LatestFailure.RunID,
			FailureCode:    strings.TrimSpace(evidence.LatestFailure.FailureCode),
		}
	}

	if strings.TrimSpace(shop.SharedLoginStatus) != "ready" {
		return OperationTaskRecord{
			TaskID:         buildOperationTaskID(shopID, "credential"),
			ShopID:         shopID,
			ShopName:       shopName,
			TaskType:       "credential_rebind",
			Title:          fmt.Sprintf("处理 %s 的店铺登录态", shopName),
			Status:         "blocked",
			BlockedReason:  firstNonEmpty(strings.TrimSpace(shop.SharedLoginStatusLabel), "店铺登录态未就绪"),
			UpdatedAt:      latestEvidenceTime(evidence),
			FailureMessage: "",
		}
	}

	return OperationTaskRecord{
		TaskID:    buildOperationTaskID(shopID, "daily_check"),
		ShopID:    shopID,
		ShopName:  shopName,
		TaskType:  "daily_check",
		Title:     fmt.Sprintf("检查 %s 今日运营状态", shopName),
		Status:    "waiting",
		UpdatedAt: latestEvidenceTime(evidence),
	}
}

func operationTaskTypeForRun(run *RunRecord) string {
	if run == nil {
		return "shop_operation"
	}
	switch strings.TrimSpace(run.TaskType) {
	case "open":
		return "shop_open"
	case "bind":
		return "credential_rebind"
	case "validate":
		return "session_validate"
	default:
		return firstNonEmpty(strings.TrimSpace(run.TaskType), "shop_operation")
	}
}

func matchesOperationTaskQuery(task OperationTaskRecord, query OperationTaskQuery) bool {
	if strings.TrimSpace(query.ShopID) != "" && task.ShopID != strings.TrimSpace(query.ShopID) {
		return false
	}
	if strings.TrimSpace(query.Status) != "" && task.Status != strings.TrimSpace(query.Status) {
		return false
	}
	if strings.TrimSpace(query.TaskType) != "" && task.TaskType != strings.TrimSpace(query.TaskType) {
		return false
	}
	return true
}

func operationTaskStatusRank(status string) int {
	switch status {
	case "running":
		return 0
	case "failed":
		return 1
	case "blocked":
		return 2
	case "waiting":
		return 3
	case "completed":
		return 4
	default:
		return 5
	}
}

func buildOperationTaskID(shopID string, suffix string) string {
	return fmt.Sprintf("op:%s:%s", strings.TrimSpace(shopID), firstNonEmpty(strings.TrimSpace(suffix), "task"))
}

func latestEvidenceTime(evidence ShopRunEvidence) string {
	for _, run := range []*RunRecord{
		evidence.ActiveRun,
		evidence.LatestFailure,
		evidence.LatestOpen,
		evidence.LatestValidation,
		evidence.LatestCredential,
	} {
		if run == nil {
			continue
		}
		if value := firstNonEmpty(run.FinishedAt, run.StartedAt); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
