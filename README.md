# [tanglebeat.com](http://tanglebeat.com)


### Table of contents
- [About Tanglebeat](#about_tanglebeat)
- [What it can be usefull for?](#what_it_can_be_useful_for)
- [Architecture](#architecture)
- [Contents of the repository](#contents_of_the_repository)
- [Getting started](#getting_started)
    * [Download and install](#download_and_install)
    * [Configure](#configure)
    * [Run](#run)
- [Advanced configurations](#advanced_configurations)

## About Tanglebeat
**Tanglebeat** is a configurable software agent with the main purpose to 
collect IOTA Tangle - related metrics to [Prometheus TSDB](https://prometheus.io/) to be displayed with such tools
as [Grafana](https://grafana.com). Apart from being a metrics collector, Tanglebeat can be used for other purposes.

It can be run in various standalone and distributed configurations to ensure 
high availability and objectivity of the metrics. 

Tanglebeat is a result of experimenting with different approaches to how to measure a Tangle in 
objective and reliable way.


### What it can be useful for?
The functions of the Tanglebeat are:

##### A hub for IRI ZMQ streams
Tanglebeat  collects messages from many IRI ZMQ sources and produces 
one output message stream which represents operational state of the network as a whole 
(as opposed to the ZMQ stream from a specific node). 

ZMQ sources can change on the fly, some nodes going down and/or going out of sync,
other nodes restarting and so on. Typically Tanglebeat is listening to 50 and more IRI nodes.
Tanglebeat combines incoming data by using some kind of quorum based algo 
(message is passed to the output only after received from several different sources).

Applications which rely on IRI ZMQ data may use Tanglebeat as ZMQ source, independent 
from any IRI node Output message streams is exposed by using 
[Nanomsg](https://nanomsg.org/), equivalent to ZMQ but technically 
more advanced in many cases.


##### Metrics collector for ZMQ-based metrics 
These are usual metrics like *TPS*, *CTPS*, *Conf.rate*, duration between milestones. 
Tanglebeat also collects value based metrics such as number of confirmed bundles, 
value of confirmed transactions/bundles and similar.

Tanglebeat submits all metrics to the [Prometheus](https://prometheus.io/) instance of your choice.
*Prometheus* is times series database intended for collections time series data 
(timed sequences of floating point numbers). *Prometheus* provides REST API and PromQL formula 
language to access stored time series data in every imaginable way.

Prometheus is often used as a data source for [Grafana](https://grafana.com), a tool to display 
metrics in user friendly way: charts, gauges and similar.

##### Metrics collector for non-ZMQ metrics 
Some useful IOTA metrics can't be calculated from ZMQ data or it is more practical to collect it
in in other way. These metrics are submitted to *Prometheus* as well.

Tanglebeat does active sending of funds in IOTA network and calculates transfer statistics: 
transfers per hour, average transfer time, interval of expected transfer time and others. 
It does it by sending and confirming/promoting few iotas worth transfers from one account to 
another in the endless loop, along addresses of the same seed. Several seeds (sequences are used). 

- *Transfers Per Hour* or TfPH metrics is calculated the following way:
    * *(number if confirmed transfers completed in last hour in all sequences* / 
    *(average number of active sequences in the last hour)*
    
    Average number of active sequences is used to adjust for possible downtimes 
   
- *Average transfer time* is calculated from transfer statistics.
- Transfer confirmation time is estimatad by taking 25 and 75 percentiles by real 
transfer confirmation in last hour.
- *Network latency*. Promotion transactions are sent to the network to promote transfers. 
Tanglebeat records time when sent transaction returns from one of ZMQ streams back




#-----------------------------


## Transfer confirmation metrics

Tanglebeat is performing IOTA value transfers from one address 
to the next one in the endless loop.  
Tangle beat makes sure the bundle of the transfer is confirmed by _promoting_
it and _reattaching_ (if necessary). 
Tanglebeat starts sending 
whole balance of iotas to the next address in the sequence immediately after current transfer is confirmed. And so on.

Several sequences of addresses are run in parallel. 
Confirmation time and other statistical data is collected in 
the process of sending and, after some averaging, is provided as 
metrics. 

Examples of the metrics are _transfers per hour or TfPH_, _average PoW cost per confirmed transfer_, _average confirmation time_.

Tanglebeat publishes much more data about transfers through the open message channel. It can be used to calculate other metrics and to visualize transfer progress.

## ZeroMQ metrics

Tanglebeat also provides usual metrics derived from data of Zero MQ stream by IRI such as _TPS_, _CTPS_, _Confirmation rate_ and _duration between milestones_

## Configure and run

Tanglebeat is a single binary. It takes no command line arguments. Each Tanglebeat instance is configured 
through  [tanglebeat.yml](cfg_example.yml) file which must be located in the working 
directory where instance ir run. The config file contains seeds of sequences therefore should never be made public.

Each Tanglebeat instance consists of the following parts. Every part can be enabled, disabled and configured
independently from each other thus enabling congfiguration of any size and complexity.
- Sender
- Update collector
- Prometheus metrics collectors. It consists _sender metrics_ part and _ZMQ metrics_ part.

See [picture in the end](#Picture)

#### Sender

_Sender_ is running sequences of transfers. It generates transfer bundles, promotion and 
(re)attaches them until confirmed for each of enabled sequences of addresses. 
Sender is configured through `sender` section in the config file. It contains global and individual parameters 
for each sequence of transfers.
```
sender:
    enabled: true
    globals:
        iotaNode: &DEFNODE https://field.deviota.com:443    
        iotaNodeTipsel: *DEFNODE              
        iotaNodePOW: https://api.powsrv.io:443
        apiTimeout: 15
        tipselTimeout: 15
        powTimeout: 15
        txTag: TANGLE9BEAT
        txTagPromote: TANGLE9BEAT
        forceReattachAfterMin: 15
        promoteEverySec: 10
        
    sequences:
        Sequence 1:
            enabled: true
            iotaNode: http://node.iotalt.com:14600
            seed: SEED99999999999999
            index0: 0
            promoteChain: true
        Another sequence:
            enabled: false
            seed: SEED9ANOTHER9999999999999
            index0: 0
            promoteChain: true
        # more sequences can be configured    
```   
Sender generates **updates** with the information 
about the progress, per sequence. Each update [contains all the data](https://github.com/lunfardo314/tanglebeat/blob/baf8c69bc119e5ba854d0d28a8746df94f1d318b/sender_update/types.go#L22), 
about the event. Metrics are calculated from sender updates. 
It also allows visualisation of the sendinf process like [Traveling IOTA](http://traviota.iotalt.com) does.
 
#### Update collector

If enabled, update collector gathers updates from one or many 
senders (sources) into one resulting stream of updates. This function allows to configure distributed network of Tanglebeat agents.
The sender itself is on of sources.
Data stream from update collector is used to calculate metrics for Prometheus.

```
senderUpdateCollector: 
    # sender update collector is enabled if at least one of sources is enabled
    # sender update collector collects updates from one or more sources to one stream
    sources: 
        # sources with the name 'local' means that all updates generated by this instance are collected into 
        # resulting stream. If 'local' source is disabled, it means that localy generated updates 
        # have no effect on metrics and are not published
        local:
            enabled: true
        # unlimited number of external tanglebeat instances can be specified. 
        # Each source is listened for published updates and collected into the resulting stream. 
        tanglebeat2:
            enabled: true
            target: "tcp://node.iotalt.com:3100"
    # if true, resulting stream of updates is published to outside through specified port
    # if false, resulting stream is not published and only used to calculate the metrics
    publish: true
    outPort: 3100
```

If `publish=true` resulting stream of updates is exposed through specified ports and can be collected by other 
Tanglebeat instances. Data is published over [Nanomsg/Mangos](https://github.com/nanomsg/mangos) 
sockets in the form of [JSON messages](https://github.com/lunfardo314/tanglebeat/blob/baf8c69bc119e5ba854d0d28a8746df94f1d318b/sender_update/types.go#L22).

Published sender updates can be subscribed by external consumers for expample in order to calculate own metrics. 

[Here's an example](https://github.com/lunfardo314/tanglebeat/tree/ver0/examples/statsws) of a client (in Go) which calculates averaged confirmation time statistics to expose it as web service


#### Prometheus metrics collectors
If enabled, it exposes metrics to Prometheus. There are two 
independent parts:
- _Sender metrics_. It exposes metrics calculated from sender update stream: 
    - `tanglebeat_confirmation_counter`, `tanglebeat_pow_cost_counter`, `anglebeat_confirmation_duration_counter`,
    `tanglebeat_pow_duration_counter`, `tanglebeat_tipsel_duration_counter`
    - and metrics precalculated by Prometheus rules in [tanglebeat.rules](tanglebeat.rules)

- _Zero MQ metrics_. If enabled, reads Zero MQ from IRI node, calculates and exposes 
   TPS, CTPS etc metrics to Prometheus.

#### Picture

![Picture of the main parts of Tanglebeat](tanglebeat.png)


### Introduction


