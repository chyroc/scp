package scp

import (
	"path/filepath"
	"strings"
)

func (r *SCP) UploadFile(src, dest string, opt *Option) error {
	if opt == nil {
		opt = new(Option)
	}

	opt.log("src=%q, dest=%q", src, dest)

	return r.uploadAnyFile(src, dest, opt)
}

func (r *SCP) uploadAnyFile(src, dest string, opt *Option) error {
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

	return p.runScp(s, src, dest, []byte("-t"), func() error {
		if err := p.readResp("start"); err != nil {
			return err
		}

		return p.uploadAnyFile(r.client, src, dest, opt)
	})
}

func joinPath(a, b string) string {
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}
