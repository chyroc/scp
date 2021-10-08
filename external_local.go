package scp

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
)

func localGetFileMd5(file string) (string, error) {
	bs, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	r := md5.New()
	r.Write(bs)
	return fmt.Sprintf("%x", r.Sum(nil)), nil
}
