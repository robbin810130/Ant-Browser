package workspace

import "time"

type ShopRunEvidence struct {
	LatestOpen       *RunRecord `json:"latestOpen,omitempty"`
	LatestCredential *RunRecord `json:"latestCredential,omitempty"`
	LatestValidation *RunRecord `json:"latestValidation,omitempty"`
	LatestFailure    *RunRecord `json:"latestFailure,omitempty"`
	ActiveRun        *RunRecord `json:"activeRun,omitempty"`
}

type RunEvidenceIndex struct {
	ByShop map[string]ShopRunEvidence `json:"byShop"`
}

func BuildRunEvidenceIndex(runs []RunRecord) RunEvidenceIndex {
	index := RunEvidenceIndex{ByShop: map[string]ShopRunEvidence{}}
	for i := range runs {
		run := runs[i]
		if run.ShopID == "" {
			continue
		}
		current := index.ByShop[run.ShopID]
		switch run.TaskType {
		case "open":
			current.LatestOpen = newerRun(current.LatestOpen, &run)
		case "bind":
			current.LatestCredential = newerRun(current.LatestCredential, &run)
		case "validate":
			current.LatestValidation = newerRun(current.LatestValidation, &run)
		}
		if isFailureRun(run) {
			current.LatestFailure = newerRun(current.LatestFailure, &run)
		}
		if isActiveRun(run) {
			current.ActiveRun = newerRun(current.ActiveRun, &run)
		}
		index.ByShop[run.ShopID] = current
	}
	return index
}

func isFailureRun(run RunRecord) bool {
	return run.Status == "failed" || run.FailureCode != "" || run.FailureMessage != ""
}

func isActiveRun(run RunRecord) bool {
	switch run.Status {
	case "succeeded", "failed":
		return false
	default:
		return run.Status != ""
	}
}

func newerRun(current *RunRecord, candidate *RunRecord) *RunRecord {
	if candidate == nil {
		return current
	}
	if current == nil {
		return cloneRunRecord(candidate)
	}
	if parseRunTime(candidate.StartedAt).After(parseRunTime(current.StartedAt)) {
		return cloneRunRecord(candidate)
	}
	return current
}

func cloneRunRecord(run *RunRecord) *RunRecord {
	if run == nil {
		return nil
	}
	cloned := *run
	if run.Runtime != nil {
		runtime := *run.Runtime
		cloned.Runtime = &runtime
	}
	return &cloned
}

func parseRunTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
