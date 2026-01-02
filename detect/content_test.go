package detect

import (
	"regexp"
	"testing"
)

func TestDetector_Detect(t *testing.T) {
	tests := []struct {
		name     string
		content  Content
		detector Detector
		want     int
	}{
		{
			name: "detects matching content",
			content: Content{
				Key:      "test-file",
				Data:     []byte("my key is AKIAIOSFODNN7EXAMPLE"),
				Location: []string{"repo", "file.txt"},
			},
			detector: DefaultDetector,
			want:     1,
		},
		{
			name: "no matches in clean content",
			content: Content{
				Key:      "test-file",
				Data:     []byte("this is just regular code with no secrets"),
				Location: []string{"repo", "file.txt"},
			},
			detector: DefaultDetector,
			want:     0,
		},
		{
			name: "empty content returns no findings",
			content: Content{
				Key:      "empty",
				Data:     []byte(""),
				Location: []string{},
			},
			detector: DefaultDetector,
			want:     0,
		},
		{
			name: "custom detector with single rule",
			content: Content{
				Key:      "test",
				Data:     []byte("password123"),
				Location: []string{"file"},
			},
			detector: Detector{
				Rules: []Rule{
					{
						Name:  "Password",
						Regex: regexp.MustCompile(`password\d+`),
					},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := tt.detector.Detect(tt.content)
			if len(findings) != tt.want {
				t.Errorf("Detect() returned %d findings, want %d", len(findings), tt.want)
			}
		})
	}
}

func TestDetector_Detect_FindingFields(t *testing.T) {
	content := Content{
		Key:      "test-key",
		Data:     []byte("AKIAIOSFODNN7EXAMPLE"),
		Location: []string{"repo", "path", "file.txt"},
	}

	findings := DefaultDetector.Detect(content)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	finding := findings[0]

	if finding.ContentKey != content.Key {
		t.Errorf("ContentKey = %q, want %q", finding.ContentKey, content.Key)
	}

	if len(finding.Location) != len(content.Location) {
		t.Errorf("Location length = %d, want %d", len(finding.Location), len(content.Location))
	}

	for i, loc := range finding.Location {
		if loc != content.Location[i] {
			t.Errorf("Location[%d] = %q, want %q", i, loc, content.Location[i])
		}
	}

	if finding.Match != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("Match = %q, want %q", finding.Match, "AKIAIOSFODNN7EXAMPLE")
	}
}

func TestDetector_Detect_MatchField(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		wantMatch string
	}{
		{
			name:      "AWS access key match",
			data:      "credentials: AKIAIOSFODNN7EXAMPLE in config",
			wantMatch: "AKIAIOSFODNN7EXAMPLE",
		},
		{
			name:      "GitHub token match",
			data:      "token=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			wantMatch: "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		},
		{
			name:      "match at start of content",
			data:      "AKIAIOSFODNN7EXAMPLE",
			wantMatch: "AKIAIOSFODNN7EXAMPLE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := Content{
				Key:      "test",
				Data:     []byte(tt.data),
				Location: []string{"file"},
			}

			findings := DefaultDetector.Detect(content)
			if len(findings) == 0 {
				t.Fatal("expected at least 1 finding")
			}

			if findings[0].Match != tt.wantMatch {
				t.Errorf("Match = %q, want %q", findings[0].Match, tt.wantMatch)
			}
		})
	}
}

func TestDefaultDetector_RulesMatchVerifiers(t *testing.T) {
	for _, rule := range DefaultDetector.Rules {
		t.Run(rule.Name, func(t *testing.T) {
			if rule.Regex == nil {
				t.Fatalf("rule %q has nil Regex", rule.Name)
			}

			if len(rule.RegexVerifier) == 0 {
				t.Errorf("rule %q has no verifiers", rule.Name)
				return
			}

			for i, verifier := range rule.RegexVerifier {
				if !rule.Regex.MatchString(verifier) {
					t.Errorf("regex does not match verifier[%d]: %q", i, verifier)
				}
			}
		})
	}
}
