package actor

import (
	"github.com/AsynkronIT/protoactor-go/actor"
)

type PIDAware interface {
	SetPID(name *actor.PID)
}

type PIDHolder struct {
	pid *actor.PID
}

func (ph *PIDHolder) SetPID(pid *actor.PID) {
	ph.pid = pid
}

func (ph *PIDHolder) GetSelf() *actor.PID {
	return ph.pid
}

type PIDAwarePlugin struct{}

func (p *PIDAwarePlugin) OnStart(ctx actor.Context) {
	if p, ok := ctx.Actor().(PIDAware); ok {
		p.SetPID(ctx.Self())
	}
}

func (p *PIDAwarePlugin) OnOtherMessage(ctx actor.Context, usrMsg interface{}) {}
