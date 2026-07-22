package runner

import "testing"

func TestDogegoLiveWorkflow10Doc(t *testing.T) {
	if DogegoLiveWorkflow10Doc == "" {
		t.Fatal("empty doc")
	}
	if DogegoLiveWorkflow10Doc != "docs/CORE_SIDE_BY_SIDE_WORKFLOWS.md#workflow-10-dogego-live-scheduled-ci-reboottestnet" {
		t.Fatalf("doc %q", DogegoLiveWorkflow10Doc)
	}
}
