package aposerver

import (
	"github.com/sirupsen/logrus"
	"time"
)

var ReaperInterval = 1000 * time.Second

func RunReapers(ctx *ServerContext) chan bool {
	var done chan bool
	go func() {
		logrus.Infof("Starting the background reaper thread")
		ticker := time.NewTicker(ReaperInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				logrus.Info("Running reapers")
				doRunReapers(ctx)
			case <-done:
				logrus.Info("Stopping the reaper thread")
				break
			}
		}
	}()

	return done
}

func doRunReapers(context *ServerContext) {
	err := context.TokenStore.ReapTokens(time.Now())
	if err != nil {
		logrus.Errorf("Encountered error while reaping tokens: %s", err.Error())
	} else {
		logrus.Infof("Reaped old tokens")
	}
}
