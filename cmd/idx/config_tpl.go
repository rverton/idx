package main

const tplConfig = `{
    "targets": {
        "bitbucket-cloud": {
            "example-cloud-target": {
                "username": "your-bitbucket-username",
                "apiToken": "your-app-password"
            }
        },
        "bitbucket-dc": {
            "example-dc-target": {
                "username": "your-bitbucket-username",
                "apiToken": "your-personal-access-token",
                "baseURL": "https://your.bitbucket.server.com"
            }
        },
        "confluence-dc": {
            "example-confluence-target": {
                "apiToken": "your-personal-access-token",
                "baseURL": "https://your.confluence.server.com",
                "spaceNames": ["My Space", "Another Space"],
				"disableHistorySearch": false
            }
        },
        "jira-dc": {
            "example-jira-target": {
                "apiToken": "your-personal-access-token",
                "baseURL": "https://your.jira.server.com",
                "projectKeys": ["PROJ1", "PROJ2"]
            }
        },
        "smb": {
            "example-smb-target": {
                "hostname": "your-file-server.local",
                "port": 445,
                "ntlmUser": "your-username",
                "ntlmPassword": "your-password",
                "domain": "YOURDOMAIN",
                "maxRecursiveDepth": 3
            }
        }
    }
} `
