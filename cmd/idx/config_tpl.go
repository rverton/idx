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
        }
    },
    "notifications": {
        "log": {
            "output": "idx.log"
        }
    }
} `
