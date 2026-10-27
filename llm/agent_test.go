package llm

import "testing"

func TestFormatAgentCommitUsesExistingConventions(t *testing.T) {
	data := CommitData{
		Type:        "feat",
		Scope:       "auth",
		Description: "add session expiration",
		Body:        []string{"Expire inactive sessions", "Preserve active sessions"},
	}
	want := "feat(auth): add session expiration\n\n- Expire inactive sessions\n- Preserve active sessions"
	for _, emoji := range []bool{false, true} {
		message, err := FormatAgentCommit(data, emoji)
		if err != nil {
			t.Fatal(err)
		}
		expected := want
		if emoji {
			expected = "✨ " + expected
		}
		if message != expected {
			t.Fatalf("got %q, want %q", message, expected)
		}
	}
	if data.Body[0] != "Expire inactive sessions" {
		t.Fatal("formatting mutated the caller's response")
	}
}

func TestFormatAgentCommitRejectsInvalidSubjects(t *testing.T) {
	for _, data := range []CommitData{
		{Description: "missing type"},
		{Type: "feat", Description: " "},
		{Type: "feat\nfix", Description: "multiple types"},
		{Type: "feat", Scope: "auth\nscope", Description: "multiple lines"},
		{Type: "feat", Description: "subject\nextra subject"},
		{Type: "feat", Description: "subject", Body: []string{"invalid\x00body"}},
	} {
		if _, err := FormatAgentCommit(data, true); err == nil {
			t.Fatalf("accepted invalid commit: %#v", data)
		}
	}
}
