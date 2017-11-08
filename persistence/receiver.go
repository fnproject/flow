package persistence

/**
  This is derived from vendor/github.com/AsynkronIT/protoactor-go/persistence/receiver.go
  This has been modified to support propagating event indices to plugins
*/
import (
	"reflect"

	"github.com/AsynkronIT/protoactor-go/actor"
)

// Using adds the persistence provider to a given actor
func Using(provider Provider) func(next actor.ActorFunc) actor.ActorFunc {
	return func(next actor.ActorFunc) actor.ActorFunc {
		fn := func(ctx actor.Context) {
			switch ctx.Message().(type) {
			case *actor.Started:
				next(ctx)
				if p, ok := ctx.Actor().(persistent); ok {
					p.init(provider, ctx)
				} else {
					log.Fatalf("Actor type %v is not persistent", reflect.TypeOf(ctx.Actor()))
				}
			default:
				next(ctx)
				if p, ok := ctx.Actor().(persistent); ok {
					if p.isSnapshotRequested() {
						p.sendSnapshotRequest()
					}
				}
			}
		}
		return fn
	}
}
