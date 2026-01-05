package idx

// Config represents the overall configuration for indexing targets
// I would prefer a single array for targets here, but this way we have typed target
// setting per target type.
type Config struct {
	Targets struct {
		BitbucketCloud map[string]TargetBitbucketConfig  `json:"bitbucket-cloud"`
		BitbucketDC    map[string]TargetBitbucketConfig  `json:"bitbucket-dc"`
		ConfluenceDC   map[string]TargetConfluenceConfig `json:"confluence-dc"`
		JiraDC         map[string]TargetJiraConfig       `json:"jira-dc"`
		SMB            map[string]TargetSMBConfig        `json:"smb"`
	} `json:"targets"`
}

type TargetConfluenceConfig struct {
	ApiToken             string   `json:"apiToken"`
	BaseURL              string   `json:"baseURL"`
	SpaceNames           []string `json:"spaceNames"`
	DisableHistorySearch bool     `json:"disableHistorySearch"`

	Disabled bool `json:"disabled"`
}

type TargetBitbucketConfig struct {
	Username string `json:"username"`
	ApiToken string `json:"apiToken"`
	BaseURL  string `json:"baseURL"` // unused for Bitbucket Cloud

	Workspaces []string `json:"workspaces"`

	Disabled bool `json:"disabled"`
}

type TargetJiraConfig struct {
	ApiToken    string   `json:"apiToken"`
	BaseURL     string   `json:"baseURL"`
	ProjectKeys []string `json:"projectKeys"` // important: keys != project names

	Disabled bool `json:"disabled"`
}

type TargetSMBConfig struct {
	Hostname          string `json:"hostname"`
	Port              int    `json:"port"`
	NTLMUser          string `json:"ntlmUser"`
	NTLMPassword      string `json:"ntlmPassword"`
	Domain            string `json:"domain"`
	MaxRecursiveDepth int    `json:"maxRecursiveDepth"` // 0=root folder

	Disabled bool `json:"disabled"`
}
