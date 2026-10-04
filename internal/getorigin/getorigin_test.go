package getorigin

import "testing"

func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{name: "ssh", url: "git@github.com:miqui/go-sdk-mcp-server-dev-pulse.git", want: "miqui/go-sdk-mcp-server-dev-pulse"},
		{name: "https", url: "https://github.com/miqui/go-sdk-mcp-server-dev-pulse.git", want: "miqui/go-sdk-mcp-server-dev-pulse"},
		{name: "https no git suffix", url: "https://github.com/owner/repo", want: "owner/repo"},
		{name: "trailing newline", url: "git@github.com:owner/repo.git\n", want: "owner/repo"},
		{name: "not github", url: "git@gitlab.com:owner/repo.git", wantErr: true},
		{name: "empty", url: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOwnerRepo(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseOwnerRepo(%q): got nil error, want error", tt.url)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseOwnerRepo(%q): unexpected error: %v", tt.url, err)
			}
			if got != tt.want {
				t.Errorf("ParseOwnerRepo(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
