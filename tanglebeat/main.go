package main

import (
	"flag"
	"github.com/unioproject/tanglebeat/lib/ebuffer"
	"github.com/unioproject/tanglebeat/tanglebeat/cfg"
	"github.com/unioproject/tanglebeat/tanglebeat/inputpart"
	"github.com/unioproject/tanglebeat/tanglebeat/inreaders"
	"github.com/unioproject/tanglebeat/tanglebeat/senderpart"
	"os"
	"os/exec"
	"os/signal"
	"strings"
)

// TODO clean unnecessary metrics

const CONFIG_FILE_DEFAULT = "tanglebeat.yml"

func main() {
	pcfgfile := flag.String("cfg", CONFIG_FILE_DEFAULT, "usage: tanglebeat [-cfg <config file name>]")
	flag.Parse()

	cfg.MustReadConfig(*pcfgfile)
	setLogs()
	inputpart.MustInitInputRoutines(
		cfg.Config.IriMsgStream.OutputEnabled,
		cfg.Config.IriMsgStream.OutputPort,
		cfg.Config.IriMsgStream.InputsZMQ,
		cfg.Config.IriMsgStream.InputsNanomsg)

	senderpart.MustInitSenderDataCollector(
		cfg.Config.SenderMsgStream.OutputEnabled,
		cfg.Config.SenderMsgStream.OutputPort,
		cfg.Config.SenderMsgStream.InputsNanomsg)

	initGlobStatsCollector(5)
	spawnCommands()

	chInterrupt := make(chan os.Signal, 2)
	signal.Notify(chInterrupt)
	go func() {
		<-chInterrupt
		warningf("Exiting after interrupt")
		cleanup()
		os.Exit(1)
	}()
	runWebServer(cfg.Config.WebServerPort)
}

func cleanup() {
	killCommands()
}

func setLogs() {
	SetLog(cfg.GetLog(), false)
	inreaders.SetLog(cfg.GetLog(), true)
	inputpart.SetLog(cfg.GetLog(), false)
	senderpart.SetLog(cfg.GetLog(), false)
	ebuffer.SetLog(cfg.GetLog(), false)
}

// spawning cmd lines specified in spawnCmd part of the config file
// each command is started in the separate go routine and stdout and stderr are redirected to
// the current output

var runningCmd = make([]*exec.Cmd, 0)

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
	if err := cmd.Start(); err != nil {
		errorf("Failed to run '%v' from 'tanglebeat': %v", cmdline, err)
	} else {
		runningCmd = append(runningCmd, cmd)
	}
}

func killCommands() {
	for _, cmd := range runningCmd {
		infof("Killing command %v %v", cmd.Path, cmd.Args)
		_ = cmd.Process.Kill()
	}
}
