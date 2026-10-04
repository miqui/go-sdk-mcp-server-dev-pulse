package gitx

import (
	"testing"
	"time"
)

func TestParsePorcelainV2(t *testing.T) {
	repo := Repo{Name: "example", Path: "/tmp/example"}

	tests := []struct {
		name string
		out  string
		want Status
	}{
		{
			name: "clean branch no upstream diff",
			out:  "# branch.oid abc123\n# branch.head main\n# branch.upstream origin/main\n# branch.ab +0 -0\n",
			want: Status{Repo: repo, Branch: "main", Ahead: 0, Behind: 0, Dirty: 0, Untracked: 0},
		},
		{
			name: "ahead and behind with dirty and untracked files",
			out: "# branch.head feature\n# branch.ab +2 -3\n" +
				"1 M. N... 100644 100644 100644 aaaa bbbb file1.go\n" +
				"2 R. N... 100644 100644 100644 cccc dddd file2.go\tfile2_old.go\n" +
				"? newfile.txt\n",
			want: Status{Repo: repo, Branch: "feature", Ahead: 2, Behind: 3, Dirty: 2, Untracked: 1},
		},
		{
			name: "detached head",
			out:  "# branch.head (detached)\n# branch.ab +0 -0\n",
			want: Status{Repo: repo, Branch: "(detached)", Ahead: 0, Behind: 0, Dirty: 0, Untracked: 0},
		},
		{
			name: "empty output",
			out:  "",
			want: Status{Repo: repo},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePorcelainV2(repo, tt.out)
			if got != tt.want {
				t.Errorf("parsePorcelainV2(%q) = %+v, want %+v", tt.out, got, tt.want)
			}
		})
	}
}

func TestParseForEachRef(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		want    []Branch
		wantErr bool
	}{
		{
			name: "two branches",
			out:  "main\t2024-01-15T10:30:00-05:00\nfeature/x\t2023-06-01T00:00:00Z\n",
			want: []Branch{
				{Name: "main", LastCommit: mustParse(t, "2024-01-15T10:30:00-05:00")},
				{Name: "feature/x", LastCommit: mustParse(t, "2023-06-01T00:00:00Z")},
			},
		},
		{
			name: "empty output",
			out:  "",
			want: nil,
		},
		{
			name:    "malformed line missing tab",
			out:     "main-no-date\n",
			wantErr: true,
		},
		{
			name:    "bad date",
			out:     "main\tnot-a-date\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseForEachRef(tt.out)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseForEachRef(%q): got nil error, want error", tt.out)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseForEachRef(%q): unexpected error: %v", tt.out, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseForEachRef(%q) = %+v, want %+v", tt.out, got, tt.want)
			}
			for i := range got {
				if got[i].Name != tt.want[i].Name || !got[i].LastCommit.Equal(tt.want[i].LastCommit) {
					t.Errorf("parseForEachRef(%q)[%d] = %+v, want %+v", tt.out, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("mustParse(%q): %v", s, err)
	}
	return tm
}
