package smb

import (
	"fmt"
	"time"

	"github.com/jfjallid/go-smb/smb"
	"github.com/jfjallid/go-smb/spnego"
)

var DialTimeout = 10 * time.Second

func Connect(host string, port int, user, pass string) (*smb.Connection, error) {
	smbOptions := smb.Options{
		Host: host,
		Port: port,
		Initiator: &spnego.NTLMInitiator{
			User:     user,
			Password: pass,
		},
		DialTimeout: DialTimeout,
	}

	conn, err := smb.NewConnection(smbOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SMB server: %w", err)
	}

	return conn, nil
}
