package main

import "testing"

func TestParseUpdateCLIApply(t *testing.T) {
	mode, planPath := parseUpdateCLI([]string{"ant-chrome.exe", "--apply-update", "C:/tmp/update-plan.json"})
	if mode != "apply" || planPath != "C:/tmp/update-plan.json" {
		t.Fatalf("unexpected parse result: mode=%q plan=%q", mode, planPath)
	}
}

func TestParseUpdateCLIPostCheck(t *testing.T) {
	mode, planPath := parseUpdateCLI([]string{"ant-chrome.exe", "--post-update-check", "C:/tmp/update-plan.json"})
	if mode != "post-check" || planPath != "C:/tmp/update-plan.json" {
		t.Fatalf("unexpected parse result: mode=%q plan=%q", mode, planPath)
	}
}

func TestParseUpdateCLIIgnoresMissingPlan(t *testing.T) {
	mode, planPath := parseUpdateCLI([]string{"ant-chrome.exe", "--apply-update"})
	if mode != "" || planPath != "" {
		t.Fatalf("unexpected parse result: mode=%q plan=%q", mode, planPath)
	}
}

func TestParseUpdateCLINone(t *testing.T) {
	mode, planPath := parseUpdateCLI([]string{"ant-chrome.exe"})
	if mode != "" || planPath != "" {
		t.Fatalf("unexpected parse result: mode=%q plan=%q", mode, planPath)
	}
}
