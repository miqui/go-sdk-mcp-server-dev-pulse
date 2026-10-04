package ghx

import "testing"

func TestParseRuns(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		want    int
		wantErr bool
	}{
		{
			name: "two runs",
			out: `[{"workflowName":"CI","conclusion":"success","status":"completed","createdAt":"2024-01-01T00:00:00Z","url":"https://x/1"},` +
				`{"workflowName":"CI","conclusion":"failure","status":"completed","createdAt":"2024-01-02T00:00:00Z","url":"https://x/2"}]`,
			want: 2,
		},
		{name: "empty array", out: `[]`, want: 0},
		{name: "invalid json", out: `not json`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRuns(tt.out)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseRuns(%q): got nil error, want error", tt.out)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRuns(%q): unexpected error: %v", tt.out, err)
			}
			if len(got) != tt.want {
				t.Errorf("parseRuns(%q) returned %d runs, want %d", tt.out, len(got), tt.want)
			}
		})
	}
}

func TestParsePRsAndSummarizeChecks(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string // expected StatusCheck of first PR
	}{
		{
			name: "all passing",
			out:  `[{"number":1,"title":"t","headRefName":"b","reviewDecision":"","url":"u","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"}]}]`,
			want: "passing",
		},
		{
			name: "one failing",
			out:  `[{"number":1,"title":"t","headRefName":"b","reviewDecision":"","url":"u","statusCheckRollup":[{"conclusion":"SUCCESS","status":"COMPLETED"},{"conclusion":"FAILURE","status":"COMPLETED"}]}]`,
			want: "failing",
		},
		{
			name: "pending",
			out:  `[{"number":1,"title":"t","headRefName":"b","reviewDecision":"","url":"u","statusCheckRollup":[{"conclusion":"","status":"IN_PROGRESS"}]}]`,
			want: "pending",
		},
		{
			name: "no checks",
			out:  `[{"number":1,"title":"t","headRefName":"b","reviewDecision":"","url":"u","statusCheckRollup":[]}]`,
			want: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePRs(tt.out)
			if err != nil {
				t.Fatalf("parsePRs(%q): unexpected error: %v", tt.out, err)
			}
			if len(got) != 1 {
				t.Fatalf("parsePRs(%q) returned %d PRs, want 1", tt.out, len(got))
			}
			if got[0].StatusCheck != tt.want {
				t.Errorf("parsePRs(%q)[0].StatusCheck = %q, want %q", tt.out, got[0].StatusCheck, tt.want)
			}
		})
	}
}
