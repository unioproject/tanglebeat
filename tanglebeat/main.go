package main

import (
	"flag"
	"github.com/unioproject/tanglebeat/lib/ebuffer"
	"github.com/unioproject/tanglebeat/tanglebeat/cfg"
	"github.com/unioproject/tanglebeat/tanglebeat/inputpart"
	"github.com/unioproject/tanglebeat/tanglebeat/inreaders"
	"github.com/unioproject/tanglebeat/tanglebeat/senderpart"
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
	runWebServer(cfg.Config.WebServerPort)
}

func setLogs() {
	SetLog(cfg.GetLog(), false)
	inreaders.SetLog(cfg.GetLog(), true)
	inputpart.SetLog(cfg.GetLog(), false)
	senderpart.SetLog(cfg.GetLog(), false)
	ebuffer.SetLog(cfg.GetLog(), false)
}

/*
173.249.16.125,
207.180.197.79,
188.68.58.32,
165.227.24.40,
173.249.46.93,
173.249.43.185,
173.249.34.67,
167.99.167.136,
51.158.66.236,
121.138.60.212,
193.30.120.67,
85.214.88.177,
79.13.107.8,
213.136.94.51,
206.189.64.81,
37.59.132.147,
83.169.42.179,
85.214.105.196,
185.144.100.99,
94.23.30.39,
node06.iotamexico.com,
159.69.207.216,
5.189.154.131,
207.180.228.135,
85.214.227.149,
node06.iotatoken.nl,
5.45.111.83,
159.69.54.69,
173.249.19.206,
94.16.120.108,
173.212.200.186,
173.212.204.177,
167.99.134.249,
173.249.16.62,
85.214.148.21,
173.249.42.88,
0v0.science,
207.180.231.49,
173.212.214.95,
5.189.152.244,
173.249.50.233,
185.228.137.166,
37.120.174.20,
nodes.tangled.it,
195.201.38.175,
5.189.133.22,
207.180.245.104,
163.172.180.44,
node04.iotatoken.nl,
46.163.78.156,
5.189.157.6,
node.deviceproof.org,
144.76.138.212,
173.249.52.236,
207.180.199.184,
173.249.23.161,
178.128.172.20,
89.163.242.213,
5.189.140.162,
144.76.106.187,
173.249.17.151,
173.249.16.55,
iotanode1.dyndns.biz
*/
