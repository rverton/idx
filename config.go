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
	ThrottleMs           int      `json:"throttleMs"` // minimum delay between API requests in milliseconds (default: 100)

	Disabled bool `json:"disabled"`
}

type TargetBitbucketConfig struct {
	Username string `json:"username"`
	ApiToken string `json:"apiToken"`
	BaseURL  string `json:"baseURL"` // unused for Bitbucket Cloud
	ThrottleMs int    `json:"throttleMs"` // minimum delay between API requests in milliseconds (default: 100)

	Workspaces []string `json:"workspaces"`

	Disabled bool `json:"disabled"`
}

type TargetJiraConfig struct {
	ApiToken    string   `json:"apiToken"`
	BaseURL     string   `json:"baseURL"`
	ProjectKeys []string `json:"projectKeys"` // important: keys != project names
	ThrottleMs  int      `json:"throttleMs"`  // minimum delay between API requests in milliseconds (default: 100)

	Disabled bool `json:"disabled"`
}

type TargetSMBConfig struct {
	Hostname             string `json:"hostname"`
	Port                 int    `json:"port"`
	NTLMUser             string `json:"ntlmUser"`
	NTLMPassword         string `json:"ntlmPassword"`
	Domain               string `json:"domain"`
	MaxRecursiveDepth    int    `json:"maxRecursiveDepth"`    // 0=root folder
	FolderCacheDepth     int    `json:"folderCacheDepth"`     // depth >= N uses folder-level caching (default: 2)
	FolderRescanDuration string `json:"folderRescanDuration"` // duration before re-scanning cached folders (default: 72h)
	ThrottleMs           int    `json:"throttleMs"`            // minimum delay between file operations in milliseconds (default: 0)

	Disabled bool `json:"disabled"`
}
