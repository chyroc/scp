package scp

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

func sshCreateSymbolicLink(cli *ssh.Client, source, target string) error {
	_, err := sshRunCommand(cli, fmt.Sprintf("ln -s %s %s", source, target))
	return err
}

func sshGetFileMd5(client *ssh.Client, file string) (string, error) {
	out, err := sshRunCommand(client, "md5sum "+file)
	if err != nil {
		return "", err
	}
	ss := strings.Split(out, " ")
	if len(ss) >= 2 {
		return ss[0], nil
	}
	return "", fmt.Errorf("invalid md5: %q", out)
}

func sshRunCommand(cli *ssh.Client, cmd string) (string, error) {
	s, err := cli.NewSession()
	if err != nil {
		return "", err
	}

	o := new(bytes.Buffer)
	e := new(bytes.Buffer)
	s.Stdout = o
	s.Stderr = e

	err = s.Run(cmd)
	if err != nil {
		ee := strings.TrimSpace(e.String())
		if ee != "" {
			return "", fmt.Errorf(ee)
		}
		return "", err
	}

	return o.String(), nil
}
