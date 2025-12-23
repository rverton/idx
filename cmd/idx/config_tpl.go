package main

const tplConfig = `{
    "targets": {
        "bitbucket-cloud": {
            "example-cloud-target": {
                "username": "your-bitbucket-username",
                "apiToken": "your-app-password",
            }
        },
        "bitbucket-dc": {
            "example-dc-target": {
                "username": "your-bitbucket-username",
                "apiToken": "your-personal-access-token",
                "baseURL": "https://your.bitbucket.server.com"
            }
        }
    }
} `
