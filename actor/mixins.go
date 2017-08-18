package actor

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/stream"
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

// EventStreamPlugin provides a mixin that will publish all intercepted user messages to the associated stream
type EventStreamPlugin struct {
	stream *stream.UntypedStream
}

func (p *EventStreamPlugin) OnStart(ctx actor.Context) {}

func (p *EventStreamPlugin) OnOtherMessage(ctx actor.Context, usrMsg interface{}) {
	p.stream.PID().Tell(usrMsg)
}
