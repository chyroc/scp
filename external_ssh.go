package scp

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

func createSymbolicLink(cli *ssh.Client, source, target string) error {
	_, err := runSshCommand(cli, fmt.Sprintf("ln -s %s %s", source, target))
	return err
}

func runSshCommand(cli *ssh.Client, cmd string) (string, error) {
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
