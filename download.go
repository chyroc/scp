package scp

import (
	"path/filepath"
)

func (r *SCP) DownloadFile(src, dest string, opt *Option) error {
	if opt == nil {
		opt = new(Option)
	}

	opt.log("src=%q, dest=%q", src, dest)

	return r.downloadAnyFile(src, dest, opt)
}

func (r *SCP) downloadAnyFile(src, dest string, opt *Option) error {
	src = filepath.Clean(src)
	dest = filepath.Clean(dest)

	s, err := r.client.NewSession()
	if err != nil {
		return err
	}
	p, err := newProtocolWithSession(s, opt)
	if err != nil {
		return err
	}

	return p.runScp(s, src, dest, []byte("-f"), func() error {
		if err := p.writeOk(); err != nil {
			return err
		}

		return p.downloadAnyFile(r.client, src, dest, opt)
	})
}
