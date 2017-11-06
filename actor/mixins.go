package actor

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/stream"
)

// PIDAware is an actor mix-in that makes an actor aware of its own PID
type PIDAware interface {
	SetPID(name *actor.PID)
}

// PIDHolder is a re-usable base type for actors that want to know their own PID
type PIDHolder struct {
	pid *actor.PID
}

// SetPID sets the PID on an actor via the PIDAware plugin
func (ph *PIDHolder) SetPID(pid *actor.PID) {
	ph.pid = pid
}

// GetSelf returns the actors own PID
func (ph *PIDHolder) GetSelf() *actor.PID {
	return ph.pid
}

// PIDAwarePlugin is a plugin that causes proto actor to set the PID (via PIDAware) on actors that want to know their PID
type PIDAwarePlugin struct{}

// OnStart set up the PIDAwarePlugin
func (p *PIDAwarePlugin) OnStart(ctx actor.Context) {
	if p, ok := ctx.Actor().(PIDAware); ok {
		p.SetPID(ctx.Self())
	}
}

// OnOtherMessage part of the Actor middleware interface
func (p *PIDAwarePlugin) OnOtherMessage(ctx actor.Context, usrMsg interface{}) {}

// EventStreamPlugin provides a mixin that will publish all intercepted user messages to the associated stream
type EventStreamPlugin struct {
	stream *stream.UntypedStream
}

// OnStart configures the EventST
func (p *EventStreamPlugin) OnStart(ctx actor.Context) {}

func (p *EventStreamPlugin) OnOtherMessage(ctx actor.Context, usrMsg interface{}) {
	p.stream.PID().Tell(usrMsg)
}
