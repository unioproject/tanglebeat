# [tanglebeat.com](http://tanglebeat.com)

**Tanglebeat** is a configurable software agent with the primary purpose of 
collecting IOTA Tangle-related metrics to [Prometheus Time Series Database](https://prometheus.io/). 
Metrics data later can be retrieved and displayed with such tools as [Grafana](https://grafana.com). 

Tanglebeat can be run in various standalone and distributed configurations to ensure 
high availability and objectivity of the metrics. 

Tanglebeat is a result of experimenting with different approaches to how to measure a Tangle in 
objective and reliable way.


### Contents
- [What it can be usefull for?](#what_it_can_be_useful_for)
- [Architecture](#architecture)
- [Contents of the repository](#contents_of_the_repository)
- [Download and install](#download_and_install)
- [Configure Tanglebeat](#configure_tanglebeat) ZMQ-based metrics
- [Configure Tbsender](#configure_tbsender) non-ZMQ-base metrics
- [Run](#run)
- [Advanced configurations](#advanced_configurations)
- [API](#api)


## What it can be useful for?
The functions of the Tanglebeat are:

##### A hub for IRI ZMQ streams
Tanglebeat  collects messages from many IRI ZMQ sources and produces 
one output message stream. The resulting  output represents operational state of the network as a whole 
(as opposed to one ZMQ stream from specific node).
 
Tanglebeat uses output stream to calculate metrics.

ZMQ sources can change on the fly while some nodes are going down or up, 
syncing or going out of sync, other nodes restarting and so on. 
Typically Tanglebeat is listening to 50 and more IRI nodes. Tanglebeat combines 
incoming data by using some kind of quorum based algorithm to produce healthy output stream. 
In short: a message is passed to the output only after been received from several different ZMQ sources.

Applications which rely on IRI ZMQ data may want to use Tanglebeat as ZMQ source 
independent from any specific IRI node. 

Output message stream is exposed for consumption by using 
[Nanomsg](https://nanomsg.org/) as a transport, functionally equivalent to ZMQ, in [exactly the same format as received 
from ZMQ](https://docs.iota.org/docs/iri/0.1/references/zmq-events).

(*reason why not ZMQ is used for output: there's no reliable pure Go (without C dependencies) library for ZMQ who does that. 
Nanomsg looks technically more solid than ZMQ anyway* ) 

The following types of IRI ZMQ messages are available from the output Nanomsg stream: `tx` (transaction), `sn` (confirmation),
`lmi` (latest milestone changed), `lmhs` (latest solid milestone hash). 


##### A collector of ZMQ-based metrics 
ZMQ-based metrics are usual metrics like *TPS* (transactions per second), *CTPS* (confirmed transactions per scond),
*Conf.rate* (confirmation rate), *duration between milestones*. 
Tanglebeat also collects from ZMQ value based metrics such as number of confirmed bundles, 
value of confirmed transactions/bundles and similar.

Tanglebeat submits all metrics to the [Prometheus](https://prometheus.io/) instance of your choice.
*Prometheus* is times series database intended for collections of time series data 
(timed sequences of floating point numbers). It provides REST API and PromQL formula 
language to access stored time series data in every practically imaginable way.

Prometheus is often used as a data source for [Grafana](https://grafana.com), a tool to display 
metrics in user friendly way: charts, gauges and similar.

##### A collector of non-ZMQ metrics 
Some useful IOTA metrics can't be calculated from ZMQ data or it is more practical to collect it
in other way. These metrics are submitted to *Prometheus* as well.

Tanglebeat (a separate module _tbsender_) does active sending of funds in IOTA network and calculates transfer statistics: 
transfers per hour, average transfer time, interval of expected transfer time and others. 
It does it by sending and confirming/promoting few iotas worth transfers from one account to 
another in the endless loop, along addresses of the same seed. Several seeds (sequences) are used for that. 

- *Transfers Per Hour* or TfPH metrics is calculated the following way:
    * *(number if confirmed transfers completed in last hour in all sequences*) / 
    *(average number of active sequences in the last hour)*
    
    Average number of active sequences is used to adjust for possible downtimes. 
   
- *Average transfer time* is calculated from transfer statistics.
- Transfer confirmation time is estimated by taking _25 and 75 percentiles_ of real 
transfer confirmations times in the last hour.
- *Network latency*. Promotion transactions are sent to the network to promote transfers. 
Tanglebeat records time when sent transaction returns from one of ZMQ streams back

##### A ZMQ state monitor
Tanglebeat exposes endpoint which returns states and data of all input ZMQ streams. 
Thus many nodes can be monitored at once by up/down status, 
sync status, tps, ctps and conf. rate.

## Picture

_Tanglebeat_ consists of two programs: _tanglebeat_ itself and _tbsender_. 
The former can be run alone. _tbsender_ is a separate program which does transfers
to calculate non-ZMQ metrics. It sends all necessary data to _tanglebeat_ instance 
which submits metrics to _Prometheus_.

![Tanglebeat](picture.png)

## Repository

- Directory `tanglebeat` contains Go package for the executable of main _tanglebeat_ program.
- Directory `tbsender` contains Go package for the executable of the _tbsender_.
- Directory `examples\readnano` contains example how to read output of the _Tanglebeat_ in the form 
of _Nanomsg_ data stream.
- Directory `lib` contains common part. Some of it can be used as independent packages 
in other Go projects
    * `lib/confirmer` contains library for promotion, reattachment and confirmation of any bundle.
    * `lib/multiapi` contains library for IOTA API calls performed simultaneously to 
    several nodes with automatic handling of responses. Redundant API calling is handy to
    ensure robustness of the daemon programs by using several IOTA nodes at once.
   
 
## Download and install

##### Download and install Go
You will need Go compiler and environment to compile _Tanglebeat_ binaries on your
platform.

Follow the [instructions](https://golang.org/doc/install) how to install Go. 

Make sure to define `GOPATH` environment variable to the root where all your 
Go projects and/or dependencies will land. 

The `GOPATH` directory should contain at least `src` (for sources) and `bin` 
(for executable binaries) subdirectories. 

Set `PATH` to your `GOPATH/bin`

##### Download Tanglebeat

To download _tanglebeat_ package with dependencies run:
 
`go get -d github.com/lunfardo314/tanglebeat/tanglebeat` 

To download _tbsender_ package with dependencies run: 

`go get -d github.com/lunfardo314/tanglebeat/tbsender` 
 
To download _readnano_ package run: 

`go get -d github.com/lunfardo314/tanglebeat/examples/readnano` 
 
##### Compile Tanglebeat binaries
 
Make directory `GOPATH/src/github.com/lunfardo314/tanglebeat/tanglebeat` current.

Run `go install` 
 
Make directory `GOPATH/src/github.com/lunfardo314/tanglebeat/tbsender` current.

Run `go install` 

Make directory `GOPATH/src/github.com/lunfardo314/tanglebeat/examples/readnano` current.

Run `go install` 
 
The above will produce three executable binaries in `GOPATH/bin` directory
  
## Configure and run
 
##### Configure Tanglebeat instance
Tanglebeat instance is configured with YAML config file. 

It must be located in current directory or in the directory specified by `SITE_DATA_DIR` 
environments variable. Default name is `tanglebeat.yml`. It also can be specified
with `-cfg <config file>` command line flag.

Directory `examples/config` contains [example](examples/config/tanglebeat.yml) of the config file. 
Please read instructions right in the file. In most cases you'll only need to adjust ports used
by the instance and static list of URI's of IRI ZMQs.

##### Configure Prometheus 
TBD

##### Configure TBsender
TBD


##### Run

`tanglebeat [-cfg <config file>]`


## Advanced configurations
TBD

## API
TBD


