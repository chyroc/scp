package scp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/ssh"
)

// protocol: https://itectec.com/unixlinux/ssh-the-protocol-for-sending-files-over-ssh-in-code/

const (
	respOK       = '\x00'
	respNonFatal = '\x01'
	respFatal    = '\x02'
)

const (
	reqFile     = 'C'
	reqDirStart = 'D'
	reqDirEnd   = 'E'
	reqTime     = 'T'
)

type protocol struct {
	in        io.WriteCloser
	out       io.Reader
	outReader *bufio.Reader
	opt       *Option
}

func respTypeToString(b byte) string {
	switch b {
	case respOK:
		return "Ok"
	case respNonFatal:
		return "NonFatal"
	case respFatal:
		return "Fatal"
	default:
		return "Unknown"
	}
}

func newProtocolWithSession(session *ssh.Session, opt *Option) (*protocol, error) {
	in, err := session.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}

	return newProtocol(in, out, opt)
}

func newProtocol(in io.WriteCloser, out io.Reader, opt *Option) (*protocol, error) {
	r := &protocol{
		in:        in,
		out:       out,
		opt:       opt,
		outReader: bufio.NewReader(out),
	}

	return r, nil
}

func (r *protocol) sendDir(mode os.FileMode, name string, f func() error) (finalErr error) {
	defer func() {
		if finalErr != nil {
			return
		}
		if _, err := r.in.Write([]byte(fmt.Sprintf("%c\n", reqDirEnd))); err != nil && finalErr == nil {
			finalErr = err
			return
		}
		r.log("send_dir end=E")

		if err := r.readResp("send-dir-end"); err != nil && finalErr == nil {
			finalErr = err
			return
		}
	}()

	msg := fmt.Sprintf("%c%#4o 0 %s\n", reqDirStart, mode&os.ModePerm, filepath.Base(name))
	r.log("send_dir msg=%s, from=%q", msg, name)
	if _, err := r.in.Write([]byte(msg)); err != nil {
		return err
	}

	if err := r.readResp("send-dir-start"); err != nil {
		return err
	}

	r.log("send_dir recursively")
	if err := f(); err != nil {
		return err
	}

	return nil
}

func (r *protocol) sendFile(mode os.FileMode, size int64, name string, reader io.Reader) error {
	msg := fmt.Sprintf("%c%#4o %d %s\n", reqFile, mode&os.ModePerm, size, filepath.Base(name))
	r.log("send_file send msg=%s, from=%q", msg, name)
	_, err := r.in.Write([]byte(msg))
	if err != nil {
		return err
	}

	if err := r.readResp("send-file-start"); err != nil {
		return err
	}

	n, err := io.Copy(r.in, reader)
	r.log("send_file send content, size=%d", n)
	if err != nil {
		return err
	}

	// 重要：从这一句可以看出，read EOF，并不会发送 ack 消息，只有 \n 和 \x00 \x01 \x02 才会触发 ack 消息
	// 测试过这里不 write ok，看看是否可以读到数据：读不到数据
	r.log("send_file send \\x00")
	if _, err := r.in.Write([]byte{respOK}); err != nil {
		return err
	}

	return r.readResp("send-file-end")
}

func (r *protocol) uploadAnyFile(client *ssh.Client, src, dest string, opt *Option) error {
	src = filepath.Clean(src)
	dest = filepath.Clean(dest)

	srcInfo, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		r.trigger(TriggerBeforeSendDir, src, dest, &TriggerOption{})
		err = r.sendDir(srcInfo.Mode(), src, func() error {
			fs, err := os.ReadDir(src)
			if err != nil {
				return err
			}
			for _, v := range fs {
				err = r.uploadAnyFile(client, joinPath(src, v.Name()), joinPath(dest, v.Name()), opt)
				if err != nil {
					return err
				}
			}
			return nil
		})
		r.trigger(TriggerAfterSendDir, src, dest, &TriggerOption{Err: err})
		return err
	} else if (srcInfo.Mode()&os.ModeSymlink != 0) && opt.SymbolicLink {
		r.trigger(TriggerBeforeSendFile, src, dest, &TriggerOption{})
		link, err := os.Readlink(src)
		opt.log("is_link=%q -> %q", srcInfo.Name(), link)
		if err != nil {
			r.trigger(TriggerAfterSendFile, src, dest, &TriggerOption{Err: err})
			return err
		}
		doChanged, err := sshCreateSymbolicLink(client, link, dest)
		r.trigger(TriggerAfterSendFile, src, dest, &TriggerOption{Skip: !doChanged, Err: err})
		return err
	} else {
		r.trigger(TriggerBeforeSendFile, src, dest, &TriggerOption{})
		srcFile, err := os.Open(src)
		if err != nil {
			r.trigger(TriggerAfterSendFile, src, dest, &TriggerOption{Err: err})
			return err
		}

		if opt.SkipMd5EqualFile {
			localMd5, _ := localGetFileMd5(src)
			sshMd5, _ := sshGetFileMd5(client, dest)
			// fmt.Println(src,localMd5)
			// fmt.Println(dest,sshMd5)
			// fmt.Println(sshMd5 != "" , localMd5 == sshMd5,sshMd5 != "" && localMd5 == sshMd5)
			if sshMd5 != "" && localMd5 == sshMd5 {
				r.trigger(TriggerAfterSendFile, src, dest, &TriggerOption{Skip: true})
				return nil
			}
		}

		err = r.sendFile(srcInfo.Mode(), srcInfo.Size(), src, srcFile)
		r.trigger(TriggerAfterSendFile, src, dest, &TriggerOption{Err: err})
		return err
	}
}

func (r *protocol) runScp(session *ssh.Session, src, dest string, mode []byte, f func() error) error {
	dir, _ := filepath.Split(dest)

	wg := new(sync.WaitGroup)
	wg.Add(1)

	var finalErr error
	go func() {
		defer wg.Done()
		defer r.in.Close()

		finalErr = f()
	}()

	stat, err := os.Lstat(src)
	if err != nil {
		return err
	}

	// mode := []byte("-t")
	mode = append(mode, 'p')
	if stat.IsDir() {
		mode = append(mode, 'r')
	}

	cmd := fmt.Sprintf("/usr/bin/scp %s %s", mode, dir)
	r.log("scp command: %q", cmd)
	runErr := session.Run(cmd)
	wg.Wait()

	if finalErr != nil {
		return finalErr
	}

	return runErr
}

func (r *protocol) readResp(msg string) error {
	b, err := r.outReader.ReadByte()
	if err != nil {
		if errors.Is(err, io.EOF) {
			r.log("resp[%s]: read state EOF", msg)
			return nil
		}
		r.log("resp[%s]: read state err: %s", msg, err)
		return err
	}

	switch b {
	case respOK:
		r.log("resp[%s]: success", msg)
		return nil
	case respNonFatal:
		line, _, err := r.outReader.ReadLine()
		if err != nil && !errors.Is(err, io.EOF) {
			r.log("resp[%s]: non-fatal read_line fail: %s", msg, err)
			return err
		}
		r.log("resp[%s]: non-fatal: %q", msg, line)
		return fmt.Errorf(string(line))
	case respFatal:
		line, _, err := r.outReader.ReadLine()
		if err != nil && !errors.Is(err, io.EOF) {
			r.log("resp[%s]: fatal read_line err: %s", msg, err)
			return err
		}
		r.log("resp[%s]: fatal: %q", msg, line)
		return fmt.Errorf(string(line))
	default:
		r.log("resp[%s]: unsupported %s", msg, respTypeToString(b))
		return fmt.Errorf("unsupport response type: %c", b)
	}
}

// download

func (r *protocol) downloadAnyFile(client *ssh.Client, src, dest string, opt *Option) error {
	return nil
}

func (r *protocol) readHeader() (*sshProtocolRespHeader, error) {
	b, err := r.outReader.ReadByte()
	if err != nil {
		return nil, err
	}
	switch b {
	case respOK:
		return &sshProtocolRespHeader{}, nil
	case respNonFatal, respFatal:
		m, _, err := r.outReader.ReadLine()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf(string(m))
	case reqDirStart:
	case reqDirEnd:
	case reqFile:
	case reqTime:
	default:
		return nil, fmt.Errorf("unsupport response type: %c", b)
	}

	// TODO
	return nil, err
}

type sshProtocolRespHeader struct{}

func (r *protocol) writeOk() error {
	_, err := r.in.Write([]byte{respOK})
	return err
}
