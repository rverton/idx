package main

const tplConfig = `{
    "targets": {
        "smb": {
            "example-smb-1": {
                "username": "user",
                "password": "password",
                "host": "127.0.0.1/32",
                "port": 445,
                "shares": "*",
                "exclude-shares": [
                    "share1",
                    "share2"
                ],
                "exclude-extensions": [
                    "exe",
                    "bin"
                ]
            }
        },
        "bitbucket": {
            "example-bitbucket-cloud": {
                "baseURL": "https://api.bitbucket.org/2.0"
            },
            "example-bitbucket-server": {
                "username": "your-bitbucket-username",
                "appPassword": "your-personal-access-token",
                "baseURL": "https://your.bitbucket.server.com/rest/api/1.0"
            }
        },
		"gitlab": {
			"example-gitlab-com": {
				"accessToken": "your-gitlab-personal-access-token",
				"baseURL": "https://gitlab.com/api/v4"
			}
		}
    },
} `
