package llm

import (
	"os"
	"os/exec"
	"testing"
)

// Live test against the Ark runtime. Skipped unless ARK_API_KEY and
// ARK_MODEL_ID are set: go test -v -run TestGenerateDoubaoCommitMessageLive ./llm
func TestGenerateDoubaoCommitMessageLive(t *testing.T) {
	apiKey := os.Getenv("ARK_API_KEY")
	modelID := os.Getenv("ARK_MODEL_ID")
	if apiKey == "" || modelID == "" {
		t.Skip("ARK_API_KEY / ARK_MODEL_ID not set")
	}

	diff, err := exec.Command("git", "diff", "HEAD", "--", "model.go").Output()
	if err != nil || len(diff) == 0 {
		t.Fatalf("getting diff: %v (len=%d)", err, len(diff))
	}

	msg, err := generateDoubaoCommitMessage(string(diff), apiKey, modelID, "", true)
	if err != nil {
		t.Fatalf("generateDoubaoCommitMessage: %v", err)
	}
	if msg == "" {
		t.Fatal("empty commit message returned")
	}
	t.Logf("generated commit message:\n%s", msg)
}
