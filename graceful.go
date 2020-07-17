package graceful

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"time"

	"github.com/pkg/errors"
)

// RunUntilShutdown runs the runFunc (typically a server) until a shutdown signal (like os.INTERRUPT) is received.
func RunUntilShutdown(runFunc func() error, cleanUpFunc func(context.Context) error) error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	DoAfterSignal(func(os.Signal) {
		cancelFunc()
	}, ShutdownSignals()[0], ShutdownSignals()[1:]...)
	return RunUntilCancel(ctx, runFunc, cleanUpFunc)
}

// RunUntilCancel runs something (typically a server) until the provided context receives the done signal, when the signal is received the shutdownFunc is executed. Inspired by https://medium.com/@pinkudebnath/graceful-shutdown-of-golang-servers-using-context-and-os-signals-cc1fa2c55e97
func RunUntilCancel(ctxRun context.Context, runFunc func() error, cleanUpFunc func(context.Context) error) error {
	var err error
	go func() {
		err = runFunc()
	}()
	<-ctxRun.Done()
	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err != nil {
		return errors.Wrap(err, "Encountered an error while running runFunc")
	}
	err = cleanUpFunc(ctxShutDown)
	return err
}

// DoAfterSignal executes f once any of the signals specified is received.
func DoAfterSignal(f func(os.Signal), sig1 os.Signal, sigs ...os.Signal) {
	if sigs == nil {
		sigs = []os.Signal{sig1}
	} else {
		sigs = append(sigs, sig1)
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, sigs...)
	go func() {
		s := <-c
		f(s)
	}()
}

//ShutdownSignals returns a list of the signals that shut down the application
func ShutdownSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP}
}
