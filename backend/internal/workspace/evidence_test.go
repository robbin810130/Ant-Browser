package workspace

import "testing"

func TestBuildRunEvidenceIndexSelectsLatestByShopAndTask(t *testing.T) {
	index := BuildRunEvidenceIndex([]RunRecord{
		{RunID: "old-open", ShopID: "shop-001", TaskType: "open", Status: "failed", StartedAt: "2026-05-23T00:00:00Z", FailureCode: "ANT_OPEN_FAILED"},
		{RunID: "new-open", ShopID: "shop-001", TaskType: "open", Status: "succeeded", StartedAt: "2026-05-23T00:01:00Z"},
		{RunID: "validate-1", ShopID: "shop-001", TaskType: "validate", Status: "failed", StartedAt: "2026-05-23T00:02:00Z", FailureCode: "VALIDATION_FAILED"},
	})

	shop := index.ByShop["shop-001"]
	if shop.LatestOpen == nil || shop.LatestOpen.RunID != "new-open" {
		t.Fatalf("unexpected latest open: %#v", shop.LatestOpen)
	}
	if shop.LatestValidation == nil || shop.LatestValidation.RunID != "validate-1" {
		t.Fatalf("unexpected latest validation: %#v", shop.LatestValidation)
	}
	if shop.LatestFailure == nil || shop.LatestFailure.RunID != "validate-1" {
		t.Fatalf("unexpected latest failure: %#v", shop.LatestFailure)
	}
}

func TestBuildRunEvidenceIndexTracksActiveRuns(t *testing.T) {
	index := BuildRunEvidenceIndex([]RunRecord{
		{RunID: "run-active", ShopID: "shop-001", TaskType: "open", Status: "launching", StartedAt: "2026-05-23T00:01:00Z"},
		{RunID: "run-done", ShopID: "shop-001", TaskType: "open", Status: "succeeded", StartedAt: "2026-05-23T00:00:00Z"},
	})

	shop := index.ByShop["shop-001"]
	if shop.ActiveRun == nil || shop.ActiveRun.RunID != "run-active" {
		t.Fatalf("unexpected active run: %#v", shop.ActiveRun)
	}
}

func TestBuildRunEvidenceIndexDeepCopiesRuntime(t *testing.T) {
	runtime := &RunRuntime{PID: 101, DebugPort: 9222, CurrentURL: "https://example.test/original", PageTitle: "Original"}
	runs := []RunRecord{
		{RunID: "run-open", ShopID: "shop-001", TaskType: "open", Status: "launching", StartedAt: "2026-05-23T00:01:00Z", Runtime: runtime},
	}

	index := BuildRunEvidenceIndex(runs)
	runtime.PID = 202
	runtime.DebugPort = 9333
	runtime.CurrentURL = "https://example.test/changed"
	runtime.PageTitle = "Changed"

	shop := index.ByShop["shop-001"]
	if shop.LatestOpen == nil || shop.LatestOpen.Runtime == nil {
		t.Fatalf("expected latest open runtime: %#v", shop.LatestOpen)
	}
	if shop.LatestOpen.Runtime.PID != 101 {
		t.Fatalf("unexpected runtime PID: %d", shop.LatestOpen.Runtime.PID)
	}
	if shop.LatestOpen.Runtime.DebugPort != 9222 {
		t.Fatalf("unexpected runtime debug port: %d", shop.LatestOpen.Runtime.DebugPort)
	}
	if shop.LatestOpen.Runtime.CurrentURL != "https://example.test/original" {
		t.Fatalf("unexpected runtime current URL: %q", shop.LatestOpen.Runtime.CurrentURL)
	}
	if shop.LatestOpen.Runtime.PageTitle != "Original" {
		t.Fatalf("unexpected runtime page title: %q", shop.LatestOpen.Runtime.PageTitle)
	}
}
