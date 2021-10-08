package scp

import (
	"golang.org/x/crypto/ssh"
)

type SCP struct {
	client *ssh.Client
}

type Option struct {
	Logger           Logger  // logger interface
	SymbolicLink     bool    // if true, send link file, else target file
	SkipMd5EqualFile bool    // if true, check file md5 if equal, if equal, skip upload file or download file
	Trigger          Trigger // trigger callback function to do custom logic
}

func NewSCP(client *ssh.Client) *SCP {
	return &SCP{
		client: client,
	}
}
