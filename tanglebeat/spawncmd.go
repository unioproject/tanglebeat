package main

import (
	"github.com/unioproject/tanglebeat/tanglebeat/cfg"
	"os"
	"os/exec"
	"strings"
)

// spawning cmd lines specified in spawnCmd part of the config file
// each command is started in the separate go routine and stdout and stderr are redirected to
// the current output

func spawnCommands() {
	for _, cmd := range cfg.Config.SpawnCmd {
		spawnCmd(cmd)
	}
}

func spawnCmd(cmdline string) {
	infof("Spawning command '%v' from 'tanglebeat'", cmdline)

	words := strings.Split(cmdline, " ")
	if len(words) == 0 {
		errorf("Failed to run '%v' from 'tanglebeat': wrong cmdline '%v'", cmdline)
		return
	}
	cmd := exec.Command(words[0], words[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	go func() {
		if err := cmd.Run(); err != nil {
			errorf("Failed to run '%v' from 'tanglebeat': %v", cmdline, err)
		}
	}()
}
