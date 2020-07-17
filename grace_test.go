package graceful_test

import (
	"bytes"
	"context"

	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/go-baconbits/graceful"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func ExampleRunUntilShutdown() {
	srv := http.Server{
		Addr: ":0",
	}
	graceful.RunUntilShutdown(srv.ListenAndServe, srv.Shutdown)
}

func ExampleRunUntilCancel() {
	srv := http.Server{
		Addr: ":0",
	}
	otherStuff := func() {
		time.Sleep(1 * time.Second)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go graceful.RunUntilCancel(ctx, srv.ListenAndServe, srv.Shutdown)
	otherStuff()
	cancel()
}

func TestRunUntilCancel(t *testing.T) {
	fooErr := errors.New("foo")
	fooRunErr := errors.Wrap(fooErr, "Encountered an error while running runFunc")
	testdata := []struct {
		RunErr     error
		CleanupErr error
		WantErr    error
		WantFlag   int
	}{
		{nil, nil, nil, 3},
		{nil, fooErr, fooErr, 3},
		{fooErr, nil, fooRunErr, 1},
		{fooErr, nil, fooRunErr, 1},
		{fooErr, errors.New("bar"), fooRunErr, 1},
	}
	for _, testcase := range testdata {
		t.Run("", func(t *testing.T) {
			flag := 0
			ctx, cancel := context.WithCancel(context.Background())
			run := func() error {
				flag += 1
				cancel()
				return testcase.RunErr
			}
			cleanup := func(context.Context) error {
				flag += 2
				return testcase.CleanupErr
			}
			err := graceful.RunUntilCancel(ctx, run, cleanup)
			assert.Equal(t, testcase.WantFlag, flag)
			if testcase.WantErr != nil {
				assert.EqualError(t, err, testcase.WantErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
func TestRunUntilShutdown(t *testing.T) {
	program := func() {
		do := func() error {
			fmt.Println("doing something")
			return nil
		}
		shutdown := func(context.Context) error {
			fmt.Println("shutting down")
			return nil
		}
		graceful.RunUntilShutdown(do, shutdown)
	}
	stdout, err := testAsDifferentProcess(t, program, graceful.ShutdownSignals()...)
	assert.NoError(t, err)
	assert.Equal(t, stdout, "doing something\nshutting down\n")
}

func TestTestAsDifferentProcessExitError(t *testing.T) {
	do := func() {
		fmt.Println("ABCDEFG")
		os.Exit(1)
	}
	stdout, err := testAsDifferentProcess(t, do)
	assert.Equal(t, stdout, "ABCDEFG\n")
	e, ok := err.(*exec.ExitError)
	assert.Truef(t, ok, "did not get an exit error, got '%v' instead", e)
	assert.Error(t, err)
}

func TestTestAsDifferentProcessSmooth(t *testing.T) {
	do := func() {
		fmt.Println("foo")
	}
	stdout, err := testAsDifferentProcess(t, do)
	assert.Equal(t, "foo\n", stdout)
	assert.NoError(t, err)
}

func testAsDifferentProcess(t *testing.T, do func(), sigs ...os.Signal) (string, error) {
	//based on https://stackoverflow.com/questions/26225513/how-to-test-os-exit-scenarios-in-go
	if os.Getenv("FOO") == "FORK" {
		do()
		os.Exit(0)
		return "", nil
	} else {
		buf := bytes.Buffer{}
		arg := fmt.Sprintf("-test.run=%s", t.Name())
		cmd := exec.Command(os.Args[0], arg)
		cmd.Env = append(os.Environ(), "FOO=FORK")
		cmd.Stdout = &buf
		err := cmd.Start()
		if err != nil {
			t.Fatalf("could not start process: %v", err)
		}
		if len(sigs) > 0 {
			time.Sleep(1 * time.Second)
			for _, sig := range sigs {
				cmd.Process.Signal(sig)
			}
		}
		err = cmd.Wait()
		return buf.String(), err
	}
}
