package detect

import "regexp"

type Content struct {
	Key      string
	Data     []byte
	Location []string
}

type MemoryStore struct {
	Has func(key string) bool
	Set func(key string)
}

type Finding struct {
	Rule       Rule
	ContentKey string
	Location   []string
	Match      string
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
			Regex:       regexp.MustCompile(`(?i)postgres(?:ql)?(?:\+[a-zA-Z0-9_]+)?:\/\/(?:[a-zA-Z0-9_.-]+)(?::[a-zA-Z0-9_.-]+)?@(?:[a-zA-Z0-9_.-]+)(?::\d+)?\/[a-zA-Z0-9_.-]+`),
			RegexVerifier: []string{
				`postgresql://user:password@localhost:5432/mydatabase`,
				`postgres://user@localhost/mydatabase`,
				`postgresql+psycopg2://foox:bary@a-db:5432/mydatabase`,
			},
		},
		{
			Name:        "GitHub Token",
			Description: "Detects GitHub personal access tokens and app tokens",
			Regex:       regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr|github_pat)_[a-zA-Z0-9_]{36,255}\b`),
			RegexVerifier: []string{
				`ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`,
				`github_pat_11EXAMPLE000000000000_ExampleTokenValue`,
			},
		},
		{
			Name:        "Slack Token",
			Description: "Detects Slack bot, user, and workspace tokens",
			Regex:       regexp.MustCompile(`xox[bpars]-[0-9]{10,13}-[0-9]{10,13}[a-zA-Z0-9-]*`),
			RegexVerifier: []string{
				`xoxb-1234567890-1234567890123-abcdefghijklmnop`,
				`xoxp-1234567890-1234567890123-abcdefghijklmnop`,
			},
		},
		{
			Name:        "Private Key",
			Description: "Detects PEM-encoded private keys",
			Regex:       regexp.MustCompile(`(?i)-----\s*BEGIN[ A-Z0-9_-]*PRIVATE KEY\s*-----`),
			RegexVerifier: []string{
				`-----BEGIN RSA PRIVATE KEY-----`,
				`-----BEGIN PRIVATE KEY-----`,
				`-----BEGIN EC PRIVATE KEY-----`,
			},
		},
		{
			Name:        "JWT Token",
			Description: "Detects JSON Web Tokens",
			Regex:       regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`),
			RegexVerifier: []string{
				`eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U`,
			},
		},
		{
			Name:        "Stripe API Key",
			Description: "Detects Stripe live API keys",
			Regex:       regexp.MustCompile(`[rs]k_live_[a-zA-Z0-9]{20,247}`),
			RegexVerifier: []string{
				`sk_live_1234567890abcdefghij`,
				`rk_live_abcdefghij1234567890`,
			},
		},
		{
			Name:        "SendGrid API Key",
			Description: "Detects SendGrid API keys",
			Regex:       regexp.MustCompile(`\bSG\.[a-zA-Z0-9_-]{20,24}\.[a-zA-Z0-9_-]{39,50}\b`),
			RegexVerifier: []string{
				`SG.xxxxxxxxxxxxxxxxxxxx.xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`,
			},
		},
		{
			Name:        "Twilio Account SID",
			Description: "Detects Twilio Account SIDs",
			Regex:       regexp.MustCompile(`\bAC[0-9a-f]{32}\b`),
			RegexVerifier: []string{
				`AC00000000000000000000000000000000`,
			},
		},
		{
			Name:        "MongoDB Connection String",
			Description: "Detects MongoDB connection strings with credentials",
			Regex:       regexp.MustCompile(`mongodb(?:\+srv)?://[^\s:]+:[^\s@]+@[^\s]+`),
			RegexVerifier: []string{
				`mongodb://user:password@localhost:27017/database`,
				`mongodb+srv://admin:secret@cluster.mongodb.net/mydb`,
			},
		},
		{
			Name:        "MySQL Connection String",
			Description: "Detects MySQL connection strings with credentials",
			Regex:       regexp.MustCompile(`(?i)mysql://[^\s:]+:[^\s@]+@[^\s]+`),
			RegexVerifier: []string{
				`mysql://user:password@localhost:3306/database`,
			},
		},
		{
			Name:        "NPM Token",
			Description: "Detects NPM authentication tokens",
			Regex:       regexp.MustCompile(`\bnpm_[a-zA-Z0-9]{36}\b`),
			RegexVerifier: []string{
				`npm_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`,
			},
		},
		{
			Name:        "LDAP Credentials",
			Description: "Detects LDAP bind credentials in code",
			Regex:       regexp.MustCompile(`(?i)(bind[_-]?dn|bind[_-]?password|ldap[_-]?password)[\s]*[=:][\s]*['"][^'"]+['"]`),
			RegexVerifier: []string{
				`bind_dn = "cn=admin,dc=example,dc=com"`,
				`bind_password = "secretpassword"`,
				`LDAP_PASSWORD: "mypassword"`,
			},
		},
		{
			Name:        "Exasol Connection String",
			Description: "Detects Exasol database connection strings",
			Regex:       regexp.MustCompile(`(?i)exa(?:sol)?://[^\s]+|jdbc:exa:[^\s]+`),
			RegexVerifier: []string{
				`exa://192.168.1.1:8563`,
				`jdbc:exa:192.168.6.11:8563;schema=myschema`,
				`exasol://user:pass@exasol.example.com:8563`,
			},
		},
		{
			Name:        "MSSQL Connection String",
			Description: "Detects Microsoft SQL Server connection strings",
			Regex:       regexp.MustCompile(`(?i)(?:jdbc:sqlserver://[^\s]+|Server\s*=\s*[^;]+;[^;]*(?:User\s*Id|uid)\s*=\s*[^;]+;[^;]*(?:Password|pwd)\s*=\s*[^;]+)`),
			RegexVerifier: []string{
				`jdbc:sqlserver://localhost:1433;databaseName=mydb;user=sa;password=secret`,
				`Server=myserver.database.windows.net;User Id=admin;Password=secret123;`,
			},
		},
		{
			Name:        "Airflow Fernet Key",
			Description: "Detects Apache Airflow Fernet encryption keys",
			Regex:       regexp.MustCompile(`(?i)(?:fernet[_-]?key|AIRFLOW__CORE__FERNET_KEY)[\s]*[=:][\s]*['"]?[A-Za-z0-9+/]{43}=['"]?`),
			RegexVerifier: []string{
				`fernet_key = "7T512UXSSmBOkpWimFHIVb8jK6lfmSAvx4mO6Arehnc="`,
				`AIRFLOW__CORE__FERNET_KEY=81HqDtbqAywKSOumSha3BhWNOdQ26slT6K0YaZeZyPs=`,
			},
		},
		{
			Name:        "Java KeyStore Password",
			Description: "Detects Java KeyStore passwords in configurations",
			Regex:       regexp.MustCompile(`(?i)(?:-(?:store|key)pass\s+[^\s]+|javax\.net\.ssl\.(?:keyStore|trustStore)Password\s*=\s*[^\s]+|(?:key-store-password|trust-store-password|keystore\.password|truststore\.password)\s*[=:]\s*[^\s]+)`),
			RegexVerifier: []string{
				`-storepass changeit`,
				`-keypass mysecretkey`,
				`javax.net.ssl.keyStorePassword=secret123`,
				`key-store-password: mypassword`,
				`keystore.password=changeme`,
			},
		},
		{
			Name:        "GCP Service Account Key",
			Description: "Detects Google Cloud Platform service account JSON keys",
			Regex:       regexp.MustCompile(`"type"\s*:\s*"service_account"[^}]*"private_key"`),
			RegexVerifier: []string{
				`"type": "service_account", "project_id": "my-project", "private_key": "-----BEGIN`,
			},
		},
		{
			Name:        "GitLab Token",
			Description: "Detects GitLab personal access tokens and other tokens",
			Regex:       regexp.MustCompile(`\bglpat-[0-9a-zA-Z_-]{20,}\b`),
			RegexVerifier: []string{
				`glpat-xxxxxxxxxxxxxxxxxxxx`,
				`glpat-ABC123def456GHI789jk`,
			},
		},
		{
			Name:        "GitLab OAuth Token",
			Description: "Detects GitLab OAuth application secrets",
			Regex:       regexp.MustCompile(`\bgloas-[0-9a-zA-Z_-]{20,}\b`),
			RegexVerifier: []string{
				`gloas-xxxxxxxxxxxxxxxxxxxx`,
			},
		},
		{
			Name:        "Grafana API Key",
			Description: "Detects Grafana Cloud API keys",
			Regex:       regexp.MustCompile(`\bglc_eyJ[A-Za-z0-9+/=]{60,160}\b`),
			RegexVerifier: []string{
				`glc_eyJvIjoiMTIzNDU2IiwibiI6InRlc3QiLCJrIjoiYWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY3ODkw`,
			},
		},
		{
			Name:        "Grafana Service Account Token",
			Description: "Detects Grafana service account tokens",
			Regex:       regexp.MustCompile(`\bglsa_[A-Za-z0-9]{32}_[A-Fa-f0-9]{8}\b`),
			RegexVerifier: []string{
				`glsa_aBcDeFgHiJkLmNoPqRsTuVwXyZ123456_12345678`,
			},
		},
		{
			Name:        "HashiCorp Vault Token",
			Description: "Detects HashiCorp Vault service, batch, and recovery tokens",
			Regex:       regexp.MustCompile(`\b(?:hvs|hvb|hvr)\.[a-zA-Z0-9]{24,}\b`),
			RegexVerifier: []string{
				`hvs.CAESIJNYVo9xwqVL5cX1xLbP`,
				`hvb.AAAAAQKMGPPBqhNwz7k8H1Is`,
				`hvr.CAESIJ1nQ2tBQ3JpemVyQXNm`,
			},
		},
		{
			Name:        "HashiCorp Vault Token (Legacy)",
			Description: "Detects legacy HashiCorp Vault tokens",
			Regex:       regexp.MustCompile(`\b[sbr]\.[a-zA-Z0-9]{24,}\b`),
			RegexVerifier: []string{
				`s.4qlVxaYm8wktSKLrOduP5g1F`,
				`b.AAAAAQJ6WJAJEPBOjxNNT1I2`,
			},
		},
		{
			Name:        "JDBC Connection String",
			Description: "Detects generic JDBC connection strings with credentials",
			Regex:       regexp.MustCompile(`(?i)jdbc:[a-z0-9]+://[^\s'"]+(?:user|password|pwd)=[^\s'"&]+`),
			RegexVerifier: []string{
				`jdbc:mysql://localhost:3306/db?user=admin&password=secret`,
				`jdbc:postgresql://host:5432/mydb?user=postgres&password=pass123`,
			},
		},
		{
			Name:        "PyPI Token",
			Description: "Detects PyPI API tokens",
			Regex:       regexp.MustCompile(`pypi-AgEIcHlwaS5vcmcCJ[a-zA-Z0-9_-]{50,180}`),
			RegexVerifier: []string{
				`pypi-AgEIcHlwaS5vcmcCJDM0YjY5ZmQ0LTY5YzAtNGIyZi1iZjk1LTYxN2Y5N2IyMjdmZgACKlszLCJhMjM0NTY3OC0xMjM0LTU2NzgtMTIzNC01Njc4MTIzNDU2NzgiXQAABiD`,
			},
		},
		{
			Name:        "Redis Connection String",
			Description: "Detects Redis connection strings with credentials",
			Regex:       regexp.MustCompile(`(?i)rediss?://[^\s:]+:[^\s@]+@[^\s]+`),
			RegexVerifier: []string{
				`redis://user:password@localhost:6379/0`,
				`rediss://default:mysecret@redis.example.com:6380`,
			},
		},
		{
			Name:        "Salesforce Access Token",
			Description: "Detects Salesforce OAuth access tokens",
			Regex:       regexp.MustCompile(`\b00[a-zA-Z0-9]{13}![a-zA-Z0-9_.]{96}\b`),
			RegexVerifier: []string{
				`00D5g000004XYZa!ARcAQH3dG5KmTH8fJI9nLpO2qR7sT4uV6wX8yZ0aBcDeFgHiJkLmNoPqRsTuVwXyZ.1234567890AbCdEfGhIjKlMnOpQrSt`,
			},
		},
		{
			Name:        "Webex Token",
			Description: "Detects Cisco Webex API tokens",
			Regex:       regexp.MustCompile(`(?i)webex.{0,20}\b[a-f0-9]{64}\b`),
			RegexVerifier: []string{
				`webex_token = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
				`WEBEX_API_KEY=abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789`,
			},
		},
	},
}

func (d *Detector) Detect(content Content) []Finding {
	var findings []Finding
	for _, rule := range d.Rules {
		match := rule.Regex.Find(content.Data)
		if match != nil {
			finding := Finding{
				Rule:       rule,
				ContentKey: content.Key,
				Location:   content.Location,
				Match:      string(match),
			}
			findings = append(findings, finding)
		}
	}
	return findings
}
