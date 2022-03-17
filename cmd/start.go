// Copyright 2022 The developer-guy Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	defaultInstanceName = "default"
)

var (
	flagUnix    string
	flagTcpPort string
)

// startCmd represents the run command
var startCmd = &cobra.Command{
	Use:   "start",
	Args:  cobra.ExactArgs(1),
	Short: "Starts the buildkitd in a VM and makes it accessible by given mode",
	Long: `There are two ways of starting buildkit-machine: unix and tcp.
To access buildkitd over tcp connection:

$ buildkit-machine start <instance_name> --tcp 9999
$ buildctl --addr tcp://127.0.0.1:9999 build ...

To access buildkitd over unix socket:

$ buildkit-machine start <instance_name> --unix $(pwd)/buildkitd.sock
$ buildctl --addr unix://$(pwd)/buildkitd.sock build ...
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateFlags(); err != nil {
			return err
		}

		var instName string
		if len(args) == 0 {
			instName = defaultInstanceName
		} else {
			instName = args[0]
		}

		limactlExecPath, err := exec.LookPath("limactl")
		if err != nil {
			return err
		}

		limactlCmd := exec.Command(limactlExecPath, "start", instName)
		sePipe, err := limactlCmd.StderrPipe()
		if err != nil {
			return err
		}

		err = limactlCmd.Start()
		if err != nil {
			return errors.Wrap(err, "could not run limactl")
		}

		// print the output of the subprocess
		scanner := bufio.NewScanner(sePipe)
		for scanner.Scan() {
			logrus.Info("LIMACTL ", scanner.Text())
		}

		err = limactlCmd.Wait()
		if err != nil {
			return errors.Wrap(err, "could not run limactl")
		}

		o, err := exec.Command(limactlExecPath, "show-ssh", "--format=args", instName).CombinedOutput()
		if err != nil {
			return err
		}

		sshConfigOutput := string(o)
		parts := strings.Split(sshConfigOutput, "-o")[1:]
		var sshOptions []string
		for _, p := range parts {
			if strings.Contains(p, "ControlPath") {
				continue
			}
			sshOptions = append(sshOptions, "-o", strings.TrimSpace(p))
		}

		uid, err := exec.Command(limactlExecPath, "shell", instName, "id", "-u").CombinedOutput()
		if err != nil {
			return errors.Wrap(err, "could not find user id")
		}

		var commandStr string
		if flagUnix != "" {
			sshOptions = append(sshOptions, "-nNT")
			rootlessBuildkitdSockPath := fmt.Sprintf("/run/user/%s/buildkit/buildkitd.sock", strings.TrimSpace(string(uid)))
			sockPath := fmt.Sprintf("%s:%s", flagUnix, rootlessBuildkitdSockPath)
			sshOptions = append(sshOptions, "-L", sockPath, "lima@127.0.0.1")
			commandStr = fmt.Sprintf("ssh %s", strings.Join(sshOptions, " "))
			fmt.Fprintln(os.Stdout, commandStr)
		}

		if flagTcpPort != "" {
			socatCmd := exec.Command(limactlExecPath, "shell", instName, "sudo", "apt", "install", "-y", "socat")
			if err := socatCmd.Run(); err != nil {
				return errors.Wrap(err, "could not install socat")
			}
			commandStr = fmt.Sprintf("ssh %s lima@127.0.0.1 -L %s \"socat TCP-LISTEN:9999,fork,bind=localhost UNIX-CONNECT:/run/user/%s/buildkit/buildkitd.sock\"", strings.Join(sshOptions, " "), fmt.Sprintf("%s:localhost:9999", flagTcpPort), strings.TrimSpace(string(uid)))
		}

		c := exec.Command("sh", "-c", commandStr)

		if err := c.Start(); err != nil {
			return fmt.Errorf("starting sh: %v", err)
		}

		log.Printf("%s machine started successfully.\n", instName)

		if err := c.Wait(); err != nil {
			return fmt.Errorf("starting sh: %v", err)

		}

		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

		<-signalCh

		log.Printf("%s machine stopping now...\n", instName)

		if err := exec.Command("limactl", "stop", instName, "--force").Run(); err != nil {
			return fmt.Errorf("could not stop instance: %w", err)
		}

		log.Printf("%s machine stopped successfully.\n", instName)

		if flagUnix != "" {
			_ = os.Remove(flagUnix)
		}

		return nil
	},
}

func validateFlags() error {
	if flagTcpPort == "" && flagUnix == "" {
		return errors.New("at least one scheme should be specified")
	}
	if flagTcpPort != "" && flagUnix != "" {
		return errors.New("only one scheme can be activated at the same time")
	}
	return nil
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVarP(&flagUnix, "unix", "", "", "a unix socket path for buildkitd.sock")
	startCmd.Flags().StringVarP(&flagTcpPort, "tcp", "", "", "a tcp port to bind")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// startCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
