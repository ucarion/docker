package docker

import (
	"bufio"
	"fmt"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
	"time"
)

func closeWrap(args ...io.Closer) error {
	e := false
	ret := fmt.Errorf("Error closing elements")
	for _, c := range args {
		if err := c.Close(); err != nil {
			e = true
			ret = fmt.Errorf("%s\n%s", ret, err)
		}
	}
	if e {
		return ret
	}
	return nil
}

func setTimeout(t *testing.T, msg string, d time.Duration, f func()) {
	c := make(chan bool)

	// Make sure we are not too long
	go func() {
		time.Sleep(d)
		c <- true
	}()
	go func() {
		f()
		c <- false
	}()
	if <-c && msg != "" {
		t.Fatal(msg)
	}
}

func assertPipe(input, output string, r io.Reader, w io.Writer, count int) error {
	for i := 0; i < count; i++ {
		if _, err := w.Write([]byte(input)); err != nil {
			return err
		}
		o, err := bufio.NewReader(r).ReadString('\n')
		if err != nil {
			return err
		}
		if strings.Trim(o, " \r\n") != output {
			return fmt.Errorf("Unexpected output. Expected [%s], received [%s]", output, o)
		}
	}
	return nil
}

// TestRunHostname checks that 'docker run -h' correctly sets a custom hostname
func TestRunHostname(t *testing.T) {
	stdout, stdoutPipe := io.Pipe()

	cli := docker.NewDockerCli(nil, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdRun("-h", "foobar", unitTestImageID, "hostname"); err != nil {
			t.Fatal(err)
		}
	}()

	setTimeout(t, "Reading command output time out", 2*time.Second, func() {
		cmdOutput, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if cmdOutput != "foobar\n" {
			t.Fatalf("'hostname' should display '%s', not '%s'", "foobar\n", cmdOutput)
		}
	})

	container := globalRuntime.List()[0]

	setTimeout(t, "CmdRun timed out", 10*time.Second, func() {
		<-c

		go func() {
			cli.CmdWait(container.ID)
		}()

		if _, err := bufio.NewReader(stdout).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
	})

	// Cleanup pipes
	if err := closeWrap(stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}
}

// TestRunWorkdir checks that 'docker run -w' correctly sets a custom working directory
func TestRunWorkdir(t *testing.T) {
	stdout, stdoutPipe := io.Pipe()

	cli := docker.NewDockerCli(nil, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdRun("-w", "/foo/bar", unitTestImageID, "pwd"); err != nil {
			t.Fatal(err)
		}
	}()

	setTimeout(t, "Reading command output time out", 2*time.Second, func() {
		cmdOutput, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if cmdOutput != "/foo/bar\n" {
			t.Fatalf("'pwd' should display '%s', not '%s'", "/foo/bar\n", cmdOutput)
		}
	})

	container := globalRuntime.List()[0]

	setTimeout(t, "CmdRun timed out", 10*time.Second, func() {
		<-c

		go func() {
			cli.CmdWait(container.ID)
		}()

		if _, err := bufio.NewReader(stdout).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
	})

	// Cleanup pipes
	if err := closeWrap(stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}
}

// TestRunWorkdirExists checks that 'docker run -w' correctly sets a custom working directory, even if it exists
func TestRunWorkdirExists(t *testing.T) {
	stdout, stdoutPipe := io.Pipe()

	cli := docker.NewDockerCli(nil, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdRun("-w", "/proc", unitTestImageID, "pwd"); err != nil {
			t.Fatal(err)
		}
	}()

	setTimeout(t, "Reading command output time out", 2*time.Second, func() {
		cmdOutput, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if cmdOutput != "/proc\n" {
			t.Fatalf("'pwd' should display '%s', not '%s'", "/proc\n", cmdOutput)
		}
	})

	container := globalRuntime.List()[0]

	setTimeout(t, "CmdRun timed out", 5*time.Second, func() {
		<-c

		go func() {
			cli.CmdWait(container.ID)
		}()

		if _, err := bufio.NewReader(stdout).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
	})

	// Cleanup pipes
	if err := closeWrap(stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}
}

func TestRunExit(t *testing.T) {
	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := docker.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	c1 := make(chan struct{})
	go func() {
		cli.CmdRun("-i", unitTestImageID, "/bin/cat")
		close(c1)
	}()

	setTimeout(t, "Read/Write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 15); err != nil {
			t.Fatal(err)
		}
	})

	container := globalRuntime.List()[0]

	// Closing /bin/cat stdin, expect it to exit
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}

	// as the process exited, CmdRun must finish and unblock. Wait for it
	setTimeout(t, "Waiting for CmdRun timed out", 10*time.Second, func() {
		<-c1

		go func() {
			cli.CmdWait(container.ID)
		}()

		if _, err := bufio.NewReader(stdout).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
	})

	// Make sure that the client has been disconnected
	setTimeout(t, "The client should have been disconnected once the remote process exited.", 2*time.Second, func() {
		// Expecting pipe i/o error, just check that read does not block
		stdin.Read([]byte{})
	})

	// Cleanup pipes
	if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}
}

// Expected behaviour: the process dies when the client disconnects
func TestRunDisconnect(t *testing.T) {

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := docker.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	c1 := make(chan struct{})
	go func() {
		// We're simulating a disconnect so the return value doesn't matter. What matters is the
		// fact that CmdRun returns.
		cli.CmdRun("-i", unitTestImageID, "/bin/cat")
		close(c1)
	}()

	setTimeout(t, "Read/Write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 15); err != nil {
			t.Fatal(err)
		}
	})

	// Close pipes (simulate disconnect)
	if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}

	// as the pipes are close, we expect the process to die,
	// therefore CmdRun to unblock. Wait for CmdRun
	setTimeout(t, "Waiting for CmdRun timed out", 2*time.Second, func() {
		<-c1
	})

	// Client disconnect after run -i should cause stdin to be closed, which should
	// cause /bin/cat to exit.
	setTimeout(t, "Waiting for /bin/cat to exit timed out", 2*time.Second, func() {
		container := globalRuntime.List()[0]
		container.Wait()
		if container.State.IsRunning() {
			t.Fatalf("/bin/cat is still running after closing stdin")
		}
	})
}

// Expected behaviour: the process dies when the client disconnects
func TestRunDisconnectTty(t *testing.T) {

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := docker.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	c1 := make(chan struct{})
	go func() {
		// We're simulating a disconnect so the return value doesn't matter. What matters is the
		// fact that CmdRun returns.
		if err := cli.CmdRun("-i", "-t", unitTestImageID, "/bin/cat"); err != nil {
			utils.Debugf("Error CmdRun: %s", err)
		}

		close(c1)
	}()

	setTimeout(t, "Waiting for the container to be started timed out", 10*time.Second, func() {
		for {
			// Client disconnect after run -i should keep stdin out in TTY mode
			l := globalRuntime.List()
			if len(l) == 1 && l[0].State.IsRunning() {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	// Client disconnect after run -i should keep stdin out in TTY mode
	container := globalRuntime.List()[0]

	setTimeout(t, "Read/Write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 15); err != nil {
			t.Fatal(err)
		}
	})

	// Close pipes (simulate disconnect)
	if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}

	// In tty mode, we expect the process to stay alive even after client's stdin closes.
	// Do not wait for run to finish

	// Give some time to monitor to do his thing
	container.WaitTimeout(500 * time.Millisecond)
	if !container.State.IsRunning() {
		t.Fatalf("/bin/cat should  still be running after closing stdin (tty mode)")
	}
}

// TestAttachStdin checks attaching to stdin without stdout and stderr.
// 'docker run -i -a stdin' should sends the client's stdin to the command,
// then detach from it and print the container id.
func TestRunAttachStdin(t *testing.T) {

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := docker.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		cli.CmdRun("-i", "-a", "stdin", unitTestImageID, "sh", "-c", "echo hello && cat && sleep 5")
	}()

	// Send input to the command, close stdin
	setTimeout(t, "Write timed out", 10*time.Second, func() {
		if _, err := stdinPipe.Write([]byte("hi there\n")); err != nil {
			t.Fatal(err)
		}
		if err := stdinPipe.Close(); err != nil {
			t.Fatal(err)
		}
	})

	container := globalRuntime.List()[0]

	// Check output
	setTimeout(t, "Reading command output time out", 10*time.Second, func() {
		cmdOutput, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if cmdOutput != container.ID+"\n" {
			t.Fatalf("Wrong output: should be '%s', not '%s'\n", container.ID+"\n", cmdOutput)
		}
	})

	// wait for CmdRun to return
	setTimeout(t, "Waiting for CmdRun timed out", 5*time.Second, func() {
		<-ch
	})

	setTimeout(t, "Waiting for command to exit timed out", 10*time.Second, func() {
		container.Wait()
	})

	// Check logs
	if cmdLogs, err := container.ReadLog("json"); err != nil {
		t.Fatal(err)
	} else {
		if output, err := ioutil.ReadAll(cmdLogs); err != nil {
			t.Fatal(err)
		} else {
			expectedLogs := []string{"{\"log\":\"hello\\n\",\"stream\":\"stdout\"", "{\"log\":\"hi there\\n\",\"stream\":\"stdout\""}
			for _, expectedLog := range expectedLogs {
				if !strings.Contains(string(output), expectedLog) {
					t.Fatalf("Unexpected logs: should contains '%s', it is not '%s'\n", expectedLog, output)
				}
			}
		}
	}
}

// TestRunDetach checks attaching and detaching with the escape sequence.
func TestRunDetach(t *testing.T) {

	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := docker.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		cli.CmdRun("-i", "-t", unitTestImageID, "cat")
	}()

	setTimeout(t, "First read/write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 15); err != nil {
			t.Fatal(err)
		}
	})

	container := globalRuntime.List()[0]

	setTimeout(t, "Escape sequence timeout", 5*time.Second, func() {
		stdinPipe.Write([]byte{16, 17})
		if err := stdinPipe.Close(); err != nil {
			t.Fatal(err)
		}
	})

	closeWrap(stdin, stdinPipe, stdout, stdoutPipe)

	// wait for CmdRun to return
	setTimeout(t, "Waiting for CmdRun timed out", 15*time.Second, func() {
		<-ch
	})

	time.Sleep(500 * time.Millisecond)
	if !container.State.IsRunning() {
		t.Fatal("The detached container should be still running")
	}

	setTimeout(t, "Waiting for container to die timed out", 20*time.Second, func() {
		container.Kill()
	})
}

// TestAttachDetach checks that attach in tty mode can be detached using the long container ID
func TestAttachDetach(t *testing.T) {
	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := docker.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		if err := cli.CmdRun("-i", "-t", "-d", unitTestImageID, "cat"); err != nil {
			t.Fatal(err)
		}
	}()

	var container *docker.Container

	setTimeout(t, "Reading container's id timed out", 10*time.Second, func() {
		buf := make([]byte, 1024)
		n, err := stdout.Read(buf)
		if err != nil {
			t.Fatal(err)
		}

		container = globalRuntime.List()[0]

		if strings.Trim(string(buf[:n]), " \r\n") != container.ID {
			t.Fatalf("Wrong ID received. Expect %s, received %s", container.ID, buf[:n])
		}
	})
	setTimeout(t, "Starting container timed out", 10*time.Second, func() {
		<-ch
	})

	stdin, stdinPipe = io.Pipe()
	stdout, stdoutPipe = io.Pipe()
	cli = docker.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)

	ch = make(chan struct{})
	go func() {
		defer close(ch)
		if err := cli.CmdAttach(container.ID); err != nil {
			if err != io.ErrClosedPipe {
				t.Fatal(err)
			}
		}
	}()

	setTimeout(t, "First read/write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 15); err != nil {
			if err != io.ErrClosedPipe {
				t.Fatal(err)
			}
		}
	})

	setTimeout(t, "Escape sequence timeout", 5*time.Second, func() {
		stdinPipe.Write([]byte{16, 17})
		if err := stdinPipe.Close(); err != nil {
			t.Fatal(err)
		}
	})
	closeWrap(stdin, stdinPipe, stdout, stdoutPipe)

	// wait for CmdRun to return
	setTimeout(t, "Waiting for CmdAttach timed out", 15*time.Second, func() {
		<-ch
	})

	time.Sleep(500 * time.Millisecond)
	if !container.State.IsRunning() {
		t.Fatal("The detached container should be still running")
	}

	setTimeout(t, "Waiting for container to die timedout", 5*time.Second, func() {
		container.Kill()
	})
}

// TestAttachDetachTruncatedID checks that attach in tty mode can be detached
func TestAttachDetachTruncatedID(t *testing.T) {
	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := docker.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	go stdout.Read(make([]byte, 1024))
	setTimeout(t, "Starting container timed out", 2*time.Second, func() {
		if err := cli.CmdRun("-i", "-t", "-d", unitTestImageID, "cat"); err != nil {
			t.Fatal(err)
		}
	})

	container := globalRuntime.List()[0]

	stdin, stdinPipe = io.Pipe()
	stdout, stdoutPipe = io.Pipe()
	cli = docker.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		if err := cli.CmdAttach(utils.TruncateID(container.ID)); err != nil {
			if err != io.ErrClosedPipe {
				t.Fatal(err)
			}
		}
	}()

	setTimeout(t, "First read/write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 15); err != nil {
			if err != io.ErrClosedPipe {
				t.Fatal(err)
			}
		}
	})

	setTimeout(t, "Escape sequence timeout", 5*time.Second, func() {
		stdinPipe.Write([]byte{16, 17})
		if err := stdinPipe.Close(); err != nil {
			t.Fatal(err)
		}
	})
	closeWrap(stdin, stdinPipe, stdout, stdoutPipe)

	// wait for CmdRun to return
	setTimeout(t, "Waiting for CmdAttach timed out", 15*time.Second, func() {
		<-ch
	})

	time.Sleep(500 * time.Millisecond)
	if !container.State.IsRunning() {
		t.Fatal("The detached container should be still running")
	}

	setTimeout(t, "Waiting for container to die timedout", 5*time.Second, func() {
		container.Kill()
	})
}

// Expected behaviour, the process stays alive when the client disconnects
func TestAttachDisconnect(t *testing.T) {
	stdin, stdinPipe := io.Pipe()
	stdout, stdoutPipe := io.Pipe()

	cli := docker.NewDockerCli(stdin, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	go func() {
		// Start a process in daemon mode
		if err := cli.CmdRun("-d", "-i", unitTestImageID, "/bin/cat"); err != nil {
			utils.Debugf("Error CmdRun: %s", err)
		}
	}()

	setTimeout(t, "Waiting for CmdRun timed out", 10*time.Second, func() {
		if _, err := bufio.NewReader(stdout).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
	})

	setTimeout(t, "Waiting for the container to be started timed out", 10*time.Second, func() {
		for {
			l := globalRuntime.List()
			if len(l) == 1 && l[0].State.IsRunning() {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	container := globalRuntime.List()[0]

	// Attach to it
	c1 := make(chan struct{})
	go func() {
		// We're simulating a disconnect so the return value doesn't matter. What matters is the
		// fact that CmdAttach returns.
		cli.CmdAttach(container.ID)
		close(c1)
	}()

	setTimeout(t, "First read/write assertion timed out", 2*time.Second, func() {
		if err := assertPipe("hello\n", "hello", stdout, stdinPipe, 15); err != nil {
			t.Fatal(err)
		}
	})
	// Close pipes (client disconnects)
	if err := closeWrap(stdin, stdinPipe, stdout, stdoutPipe); err != nil {
		t.Fatal(err)
	}

	// Wait for attach to finish, the client disconnected, therefore, Attach finished his job
	setTimeout(t, "Waiting for CmdAttach timed out", 2*time.Second, func() {
		<-c1
	})

	// We closed stdin, expect /bin/cat to still be running
	// Wait a little bit to make sure container.monitor() did his thing
	err := container.WaitTimeout(500 * time.Millisecond)
	if err == nil || !container.State.IsRunning() {
		t.Fatalf("/bin/cat is not running after closing stdin")
	}

	// Try to avoid the timeout in destroy. Best effort, don't check error
	cStdin, _ := container.StdinPipe()
	cStdin.Close()
	container.Wait()
}

// Expected behaviour: container gets deleted automatically after exit
func TestRunAutoRemove(t *testing.T) {
	t.Skip("Fixme. Skipping test for now, race condition")
	stdout, stdoutPipe := io.Pipe()
	cli := docker.NewDockerCli(nil, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdRun("-rm", unitTestImageID, "hostname"); err != nil {
			t.Fatal(err)
		}
	}()

	var temporaryContainerID string
	setTimeout(t, "Reading command output time out", 2*time.Second, func() {
		cmdOutput, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		temporaryContainerID = cmdOutput
		if err := closeWrap(stdout, stdoutPipe); err != nil {
			t.Fatal(err)
		}
	})

	setTimeout(t, "CmdRun timed out", 10*time.Second, func() {
		<-c
	})

	time.Sleep(500 * time.Millisecond)

	if len(globalRuntime.List()) > 0 {
		t.Fatalf("failed to remove container automatically: container %s still exists", temporaryContainerID)
	}
}

func TestCmdLogs(t *testing.T) {
	cli := docker.NewDockerCli(nil, ioutil.Discard, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	if err := cli.CmdRun(unitTestImageID, "sh", "-c", "ls -l"); err != nil {
		t.Fatal(err)
	}
	if err := cli.CmdRun("-t", unitTestImageID, "sh", "-c", "ls -l"); err != nil {
		t.Fatal(err)
	}

	if err := cli.CmdLogs(globalRuntime.List()[0].ID); err != nil {
		t.Fatal(err)
	}
}

// Expected behaviour: using / as a bind mount source should throw an error
func TestRunErrorBindMountRootSource(t *testing.T) {

	cli := docker.NewDockerCli(nil, nil, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdRun("-v", "/:/tmp", unitTestImageID, "echo 'should fail'"); err == nil {
			t.Fatal("should have failed to run when using / as a source for the bind mount")
		}
	}()

	setTimeout(t, "CmdRun timed out", 5*time.Second, func() {
		<-c
	})
}

// Expected behaviour: error out when attempting to bind mount non-existing source paths
func TestRunErrorBindNonExistingSource(t *testing.T) {

	cli := docker.NewDockerCli(nil, nil, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdRun("-v", "/i/dont/exist:/tmp", unitTestImageID, "echo 'should fail'"); err == nil {
			t.Fatal("should have failed to run when using /i/dont/exist as a source for the bind mount")
		}
	}()

	setTimeout(t, "CmdRun timed out", 5*time.Second, func() {
		<-c
	})
}

func TestImagesViz(t *testing.T) {
	stdout, stdoutPipe := io.Pipe()

	cli := docker.NewDockerCli(nil, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	image := buildTestImages(t, globalEngine)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdImages("-viz"); err != nil {
			t.Fatal(err)
		}
		stdoutPipe.Close()
	}()

	setTimeout(t, "Reading command output time out", 2*time.Second, func() {
		cmdOutputBytes, err := ioutil.ReadAll(bufio.NewReader(stdout))
		if err != nil {
			t.Fatal(err)
		}
		cmdOutput := string(cmdOutputBytes)

		regexpStrings := []string{
			"digraph docker {",
			fmt.Sprintf("base -> \"%s\" \\[style=invis]", unitTestImageIDShort),
			fmt.Sprintf("label=\"%s\\\\n%s:latest\"", unitTestImageIDShort, unitTestImageName),
			fmt.Sprintf("label=\"%s\\\\n%s:%s\"", utils.TruncateID(image.ID), "test", "latest"),
			"base \\[style=invisible]",
		}

		compiledRegexps := []*regexp.Regexp{}
		for _, regexpString := range regexpStrings {
			regexp, err := regexp.Compile(regexpString)
			if err != nil {
				fmt.Println("Error in regex string: ", err)
				return
			}
			compiledRegexps = append(compiledRegexps, regexp)
		}

		for _, regexp := range compiledRegexps {
			if !regexp.MatchString(cmdOutput) {
				t.Fatalf("images -viz content '%s' did not match regexp '%s'", cmdOutput, regexp)
			}
		}
	})
}

func TestImagesTree(t *testing.T) {
	stdout, stdoutPipe := io.Pipe()

	cli := docker.NewDockerCli(nil, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	image := buildTestImages(t, globalEngine)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdImages("-tree"); err != nil {
			t.Fatal(err)
		}
		stdoutPipe.Close()
	}()

	setTimeout(t, "Reading command output time out", 2*time.Second, func() {
		cmdOutputBytes, err := ioutil.ReadAll(bufio.NewReader(stdout))
		if err != nil {
			t.Fatal(err)
		}
		cmdOutput := string(cmdOutputBytes)
		regexpStrings := []string{
			fmt.Sprintf("└─%s Size: (\\d+.\\d+ MB) \\(virtual \\d+.\\d+ MB\\) Tags: %s:latest", unitTestImageIDShort, unitTestImageName),
			"(?m)   └─[0-9a-f]+.*",
			"(?m)    └─[0-9a-f]+.*",
			"(?m)      └─[0-9a-f]+.*",
			fmt.Sprintf("(?m)^        └─%s Size: \\d+.\\d+ MB \\(virtual \\d+.\\d+ MB\\) Tags: test:latest", utils.TruncateID(image.ID)),
		}

		compiledRegexps := []*regexp.Regexp{}
		for _, regexpString := range regexpStrings {
			regexp, err := regexp.Compile(regexpString)
			if err != nil {
				fmt.Println("Error in regex string: ", err)
				return
			}
			compiledRegexps = append(compiledRegexps, regexp)
		}

		for _, regexp := range compiledRegexps {
			if !regexp.MatchString(cmdOutput) {
				t.Fatalf("images -tree content '%s' did not match regexp '%s'", cmdOutput, regexp)
			}
		}
	})
}

func buildTestImages(t *testing.T, eng *engine.Engine) *docker.Image {

	var testBuilder = testContextTemplate{
		`
from   {IMAGE}
run    sh -c 'echo root:testpass > /tmp/passwd'
run    mkdir -p /var/run/sshd
run    [ "$(cat /tmp/passwd)" = "root:testpass" ]
run    [ "$(ls -d /var/run/sshd)" = "/var/run/sshd" ]
`,
		nil,
		nil,
	}
	image := buildImage(testBuilder, t, eng, true)

	err := mkServerFromEngine(eng, t).ContainerTag(image.ID, "test", "latest", false)
	if err != nil {
		t.Fatal(err)
	}

	return image
}

// #2098 - Docker cidFiles only contain short version of the containerId
//sudo docker run -cidfile /tmp/docker_test.cid ubuntu echo "test"
// TestRunCidFile tests that run -cidfile returns the longid
func TestRunCidFile(t *testing.T) {
	stdout, stdoutPipe := io.Pipe()

	tmpDir, err := ioutil.TempDir("", "TestRunCidFile")
	if err != nil {
		t.Fatal(err)
	}
	tmpCidFile := path.Join(tmpDir, "cid")

	cli := docker.NewDockerCli(nil, stdoutPipe, ioutil.Discard, testDaemonProto, testDaemonAddr)
	defer cleanup(globalEngine, t)

	c := make(chan struct{})
	go func() {
		defer close(c)
		if err := cli.CmdRun("-cidfile", tmpCidFile, unitTestImageID, "ls"); err != nil {
			t.Fatal(err)
		}
	}()

	defer os.RemoveAll(tmpDir)
	setTimeout(t, "Reading command output time out", 2*time.Second, func() {
		cmdOutput, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if len(cmdOutput) < 1 {
			t.Fatalf("'ls' should return something , not '%s'", cmdOutput)
		}
		//read the tmpCidFile
		buffer, err := ioutil.ReadFile(tmpCidFile)
		if err != nil {
			t.Fatal(err)
		}
		id := string(buffer)

		if len(id) != len("2bf44ea18873287bd9ace8a4cb536a7cbe134bed67e805fdf2f58a57f69b320c") {
			t.Fatalf("-cidfile should be a long id, not '%s'", id)
		}
		//test that its a valid cid? (though the container is gone..)
		//remove the file and dir.
	})

	setTimeout(t, "CmdRun timed out", 5*time.Second, func() {
		<-c
	})

}
