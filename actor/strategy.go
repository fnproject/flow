package actor

import (
	"math/rand"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/sirupsen/logrus"
)

// NewExponentialBackoffStrategy creates a new Supervisor strategy that restarts a faulting child using an exponential
// back off algorithm when decider returns actor.RestartDirective
func NewExponentialBackoffStrategy(backoffWindow time.Duration, initialBackoff time.Duration, decider actor.DeciderFunc) actor.SupervisorStrategy {
	return &exponentialBackoffStrategy{
		backoffWindow:  backoffWindow,
		initialBackoff: initialBackoff,
		decider:        decider,
		log:            logrus.WithField("logger", "supervisor_strategy"),
	}
}

type exponentialBackoffStrategy struct {
	log            *logrus.Entry
	backoffWindow  time.Duration
	initialBackoff time.Duration
	decider        actor.DeciderFunc
}

func publishFailureEvent(child *actor.PID, reason interface{}, directive actor.Directive) {
	eventstream.Publish(&actor.SupervisorEvent{
		Child:     child,
		Reason:    reason,
		Directive: directive,
	})
}

func (strategy *exponentialBackoffStrategy) HandleFailure(supervisor actor.Supervisor, child *actor.PID, rs *actor.RestartStatistics, reason interface{}, message interface{}) {
	strategy.log.WithFields(logrus.Fields{"child_id": child.Id}).Debug("Handling child actor failure")

	directive := strategy.decider(reason)

	switch directive {
	case actor.ResumeDirective:
		//resume the failing child
		publishFailureEvent(child, reason, directive)
		supervisor.ResumeChildren(child)
	case actor.RestartDirective:
		//try restart the failing child
		strategy.handleRestart(supervisor, child, rs, reason, message)

	case actor.StopDirective:
		//stop the failing child, no need to involve the crs
		publishFailureEvent(child, reason, directive)
		supervisor.StopChildren(child)
	case actor.EscalateDirective:
		//send failure to parent
		//supervisor mailbox
		//do not log here, log in the parent handling the error
		supervisor.EscalateFailure(reason, message)
	}
}

func (strategy *exponentialBackoffStrategy) handleRestart(supervisor actor.Supervisor, child *actor.PID, rs *actor.RestartStatistics, reason interface{}, message interface{}) {
	strategy.setFailureCount(rs)

	backoff := rs.FailureCount * int(strategy.initialBackoff.Nanoseconds())
	noise := rand.Intn(500)
	dur := time.Duration(backoff + noise)

	strategy.log.WithFields(logrus.Fields{"child_id": child.Id}).
		WithFields(logrus.Fields{"delay_sec": dur.Seconds()}).
		WithFields(logrus.Fields{"failure_count": rs.FailureCount}).
		Debug("Scheduling delayed child restart")

	time.AfterFunc(dur, func() {
		publishFailureEvent(child, reason, actor.RestartDirective)
		supervisor.RestartChildren(child)
	})
}

func (strategy *exponentialBackoffStrategy) setFailureCount(rs *actor.RestartStatistics) {
	rs.Fail()

	// if we are within the backoff window, exit early
	if rs.IsWithinDuration(strategy.backoffWindow) {
		return
	}

	//we are past the backoff limit, reset the failure counter
	rs.Reset()
}
