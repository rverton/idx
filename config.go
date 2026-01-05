package idx

import (
	"encoding/json"
)

type Config struct {
	Targets struct {
		BitbucketCloud map[string]TargetBitbucketConfig  `json:"bitbucket-cloud"`
		BitbucketDC    map[string]TargetBitbucketConfig  `json:"bitbucket-dc"`
		ConfluenceDC   map[string]TargetConfluenceConfig `json:"confluence-dc"`
		JiraDC         map[string]TargetJiraConfig       `json:"jira-dc"`
	} `json:"targets"`
}

type TargetConfluenceConfig struct {
	ApiToken             string   `json:"apiToken"`
	BaseURL              string   `json:"baseURL"`
	SpaceNames           []string `json:"spaceNames"`
	DisableHistorySearch bool     `json:"disableHistorySearch"`
	Disabled             bool     `json:"disabled"`
}

func (t TargetConfluenceConfig) MarshalJSON() ([]byte, error) {
	type Alias TargetConfluenceConfig
	return json.Marshal(&struct {
		*Alias
		ApiToken string `json:"apiToken"`
	}{
		Alias:    (*Alias)(&t),
		ApiToken: "REDACTED",
	})
}

// TargetBitbucketConfig defines the configuration for a Bitbucket target.
type TargetBitbucketConfig struct {
	Username string `json:"username"`
	ApiToken string `json:"apiToken"`
	BaseURL  string `json:"baseURL"` // unused for Bitbucket Cloud

	Workspaces []string `json:"workspaces"`

	Disabled bool `json:"disabled"`
}

// MarshalJSON customizes the JSON marshaling to redact the ApiToken field.
func (t TargetBitbucketConfig) MarshalJSON() ([]byte, error) {
	type Alias TargetBitbucketConfig
	return json.Marshal(&struct {
		*Alias
		ApiToken string `json:"apiToken"`
	}{
		Alias:    (*Alias)(&t),
		ApiToken: "REDACTED",
	})
}

type TargetJiraConfig struct {
	ApiToken    string   `json:"apiToken"`
	BaseURL     string   `json:"baseURL"`
	ProjectKeys []string `json:"projectKeys"`
	Disabled    bool     `json:"disabled"`
}

func (t TargetJiraConfig) MarshalJSON() ([]byte, error) {
	type Alias TargetJiraConfig
	return json.Marshal(&struct {
		*Alias
		ApiToken string `json:"apiToken"`
	}{
		Alias:    (*Alias)(&t),
		ApiToken: "REDACTED",
	})
}
