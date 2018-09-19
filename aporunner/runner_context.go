package aporunner

import (
	"apollo/proto/gen/restcli"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var NodeUpdatePeriod = 60 * time.Second

type RunnerContext struct {
	LastSuccess time.Time
	Client *restcli.Apollo
	Docker *DockerContext

	SuicideTimeout time.Duration
}

func NewRunnerContext(client *restcli.Apollo, docker *client.Client,
	suicideTimeout time.Duration) *RunnerContext {

	return &RunnerContext{
		LastSuccess:    time.Now(),
		Client:         client,
		Docker:         &DockerContext{
			Client: docker,
		},
		SuicideTimeout: suicideTimeout,
	}
}

func runWithTicker(done <- chan bool, timeout time.Duration, f func() error) {
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()
	var retry = make(chan bool, 1)
	var numRetries = 0

	// Force the first iteration immediately
	retry <- true

	for ;; {
		select {
		case <- done:
			return
		case <- ticker.C:
			break
		case <- retry:
			break
		}
		// Obtain the node info and submit it to the server
		err := f()
		if err != nil {
			numRetries++
			if numRetries < 5 {
				go func() {
					time.Sleep(timeout / 10)
					retry <- true
				}()
			}
		} else {
			numRetries = 0
		}
	}
}

func (r *RunnerContext) RunSuicider(done <- chan bool) {
	runWithTicker(done, NodeUpdatePeriod, func() error {
		if r.SuicideTimeout != 0 && r.LastSuccess.Add(r.SuicideTimeout).Before(time.Now()) {
			// Suicide timeout elapsed!
			CommitSuicide()
		}
		return nil
	})
}

// Hard-stop the node (in case the master has been offline for too long)
func CommitSuicide() {
	err := syscall.Exec("/sbin/shutdown", []string{"shutdown", "-P"}, []string{})
	// Normally Exec replaces the current process
	if err != nil {
		panic(err)
	}
}

func (r *RunnerContext) RunNodeInfoPusher(done <- chan bool) {
	runWithTicker(done, NodeUpdatePeriod, func() error {
		err := SubmitNodeInfo(r)
		if err != nil {
			logrus.Errorf("failed to send node update: %s", err.Error())
			return err
		} else {
			r.LastSuccess = time.Now()
		}
		return nil
	})
}

func (r *RunnerContext) RunTaskPoller(done <- chan bool) {
	// Poll the server for changes in task assignments
	for ;; {
		select {
		case <- done:
			return
		default:
		}
	}
}

func (r *RunnerContext) RunUntilDone() error {
	// Setup the sigint handler
	var interrupt = make(chan os.Signal)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)

	var doneSuicider = make(chan bool)
	var donePusher = make(chan bool)
	//var donePoller = make(chan bool)
	if r.SuicideTimeout != 0 {
		logrus.Infof("Starting the watchdog, timeout is %d sec.",
			r.SuicideTimeout/time.Second)
		go r.RunSuicider(doneSuicider)
	} else {
		logrus.Infof("Watchdog is disabled")
	}
	logrus.Info("Starting the node state publisher")
	go r.RunNodeInfoPusher(donePusher)

	// Wait for the OS interrupt
	<- interrupt
	logrus.Info("Interrupt received, shutting down")

	donePusher <- true
	if r.SuicideTimeout != 0 {
		doneSuicider <- true
	}
	//donePoller <- true

	return nil
}
