/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"

	"knative.dev/pkg/test/helpers"
)

const (
	EmptyCommandErrMsg = "command cannot be empty"
	DefaultErrCode = 1
	Separator = "\n"
)

// RunCommand will run the command and return the standard output, plus error if there is one.
func RunCommand(cmdLine string) (string, error) {
	cmdSplit := strings.Fields(cmdLine)
	if len(cmdSplit) == 0 {
		return "", &CommandLineError{
			Command: cmdLine,
			ErrorOutput: []byte(EmptyCommandErrMsg),
			ErrorCode: DefaultErrCode,
		}
	}

	cmdName := cmdSplit[0]
	args := cmdSplit[1:]
	cmd := exec.Command(cmdName, args...)
	var eb bytes.Buffer
	cmd.Stderr = &eb

	out, err := cmd.Output()
	if err != nil {
		err = &CommandLineError{
			Command: cmdLine,
			ErrorOutput: eb.Bytes(),
			ErrorCode: getErrorCode(err),
		}
	}

	return string(out), err
}

// RunCommands will run the commands sequentially.
// If there is an error when running a command, it will return directly with all standard output so far and the error.
func RunCommands(cmdLines ...string) (string, error) {
	var outputs []string
	for _, cmdLine := range cmdLines {
		output, err := RunCommand(cmdLine)
		outputs = append(outputs, output)
		if err != nil {
			return strings.Join(outputs, Separator), err
		}
	}
	return strings.Join(outputs, Separator), nil
}

// RunCommandsInParallel will run the commands in parallel.
// It will always finish running all commands, and return all standard output and errors together.
func RunCommandsInParallel(cmdLines ...string) (string, error) {
	errCh := make(chan error, len(cmdLines))
	outputCh := make(chan string, len(cmdLines))
	mx := sync.Mutex{}
	wg := sync.WaitGroup{}
	for i := range cmdLines {
		cmdLine := cmdLines[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			output, err := RunCommand(cmdLine)
			mx.Lock()
			outputCh <- output
			errCh <- err
			mx.Unlock()
		}()
	}

	wg.Wait()
	close(outputCh)
	close(errCh)

	os := make([]string, 0, len(cmdLines))
	es := make([]error, 0, len(cmdLines))
	for o := range outputCh {
		os = append(os, o)
	}
	for e := range errCh {
		es = append(es, e)
	}

	return strings.Join(os, Separator), helpers.CombineErrors(es)
}

// getErrorCode extracts the exit code of an *ExitError type
func getErrorCode(err error) int {
	errorCode := DefaultErrCode
	if exitError, ok := err.(*exec.ExitError); ok {
		errorCode = exitError.ExitCode()
	}
	return errorCode
}
