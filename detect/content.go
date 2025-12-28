package detect

import "regexp"

type Content struct {
	Key      string
	Data     []byte
	Location []string
}

type Finding struct {
	Rule       Rule
	ContentKey string
	Location   []string
}

type Rule struct {
	Name          string
	Description   string
	Regex         *regexp.Regexp
	RegexVerifier []string // regex patterns for verification
}

type Detector struct {
	Rules []Rule
}

var DefaultDetector = Detector{
	Rules: []Rule{
		{
			Name:        "AWS Access Key",
			Description: "Detects AWS Access Keys",
			Regex:       regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
			RegexVerifier: []string{
				`AKIAS0000000000000000`,
			},
		},
		{
			Name:        "AWS Secret Key",
			Description: "Detects AWS Secret Keys",
			Regex:       regexp.MustCompile(`(?i)aws(.{0,20})?(?-i)['"][0-9a-zA-Z/+]{40}['"]`),
			RegexVerifier: []string{
				`aws_secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"`,
			},
		},
		{
			Name:        "Google API Key",
			Description: "Detects Google API Keys",
			Regex:       regexp.MustCompile(`AIza[0-9A-Za-z-_]{35}`),
			RegexVerifier: []string{
				`AIzaSyD-EXAMPLEKEY1234567890abcdefgHIJKLMN`,
			},
		},
		{
			Name:        "Generic API Key",
			Description: "Detects Generic API Keys",
			Regex:       regexp.MustCompile(`(?i)(api[_-]?key|secret|token)(.{0,20})?['"][0-9a-zA-Z]{32,45}['"]`),
			RegexVerifier: []string{
				`api_key = "1234567890abcdef1234567890abcdef"`,
			},
		},
		{
			Name:        "PostgreSQL Connection String",
			Description: "Detects PostgreSQL connection strings",
			Regex:       regexp.MustCompile(`(?i)postgres(?:ql)?:\/\/(?:[a-zA-Z0-9_.-]+)(?::[a-zA-Z0-9_.-]+)?@(?:[a-zA-Z0-9_.-]+)(?::\d+)?\/[a-zA-Z0-9_.-]+`),
			RegexVerifier: []string{
				`postgresql://user:password@localhost:5432/mydatabase`,
				`postgres://user@localhost/mydatabase`,
			},
		},
	},
}

func (d *Detector) Detect(content Content) []Finding {
	var findings []Finding
	for _, rule := range d.Rules {
		if rule.Regex.Match(content.Data) {
			finding := Finding{
				Rule:       rule,
				ContentKey: content.Key,
				Location:   content.Location,
			}
			findings = append(findings, finding)
		}
	}
	return findings
}
