package scp

type Trigger func(typ TriggerType, src, dest string, opt *TriggerOption)

type TriggerType int

type TriggerOption struct {
	Skip bool
	Err  error
}

const (
	TriggerBeforeSendFile TriggerType = iota + 1
	TriggerAfterSendFile
	TriggerBeforeSendDir
	TriggerAfterSendDir
)

func (r *protocol) trigger(typ TriggerType, src, dest string, opt *TriggerOption) {
	if r.opt.Trigger == nil {
		return
	}
	r.opt.Trigger(typ, src, dest, opt)
}
