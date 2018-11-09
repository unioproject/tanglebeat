## About Tanglebeat
**Tanglebeat** is a lightweight yet highly configurable software agent with the main purpose to collect Tangle-related metrics to 
[Prometheus TSDB](https://prometheus.io/) to be displayed with such tools
as [Grafana](https://grafana.com). 

It can be run in various standalone and distributed configurations to ensure 
high availability and objectivty of the metrics.

Demo Grafana dashboard with testing configuration behind (two instances of Tangelbeat) can be found at [tanglebeat.com](http://tanglebeat.com:3000/d/85B_28aiz/tanglebeat-demo?refresh=10s&orgId=1&from=1541747401598&to=1541769001598&tab=general)

Tanglebeat is a successor of [Traveling IOTA](http://traviota.iotalt.com) project, 
scaled up and, hopefully, more practical version of the latter.

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

Tanglebeat publishes much more data about transfers thorough the open message channel. It can be used to calculate other metrics and to visualize transfer progress.

## ZeroMQ metrics

Tanglebeat also provides usual metrics derived from data of Zero MQ stream by IRI such as _TPS_, _CTPS_, _Confirmation rate_ and _duration between milestones_

## Highly configurable

Each Tanglebeat agent consist of the following functional 
parts. Every part can be enabled, disabled and configured
independently from each other thus enabling congfiguration of any size and complexity.
- Sender
- Update collector
- Update publisher
- Prometheus metrics collectors. It consists _sender metrics_ part and _ZMQ metrics_ part.

Tanglebeat is single binary. It takes no command line argumenst. Each Tanglebeat instance is configured through  [tanglebeat.yml](tanglebeat.yml). File which must be located in the working directory of the instance.

#### Sender

_Sender_ performs transfer bundle generation, promotion and 
(re)attachment until confirmed for each of enabled address sequences. 
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
```   
Sender generates **updates** with the information 
about the progress, per sequence. Updates contain all the data, 
necessary to calculate various metrics. 
It also allows visualisation of the process 
like [Traveling IOTA](http://traviota.iotalt.com) does.
 
#### Update collector

Data collector, if enabled, is a hub which collects updates from one or many 
senders into one stream. 
This function allows to configure distributed network of Tanglebeat 
agents to calculate metrics in high availability and more objective
manner.

Data collector must be enabled to provide transfer confirmation metrics
to Prometheus.

#### Data publisher

Data publisher, if enabled, publishes stream of updates collected 
by data collector to other Tanglebeat (collectors) in the form of  
JSON messages over Nanomsg/Mangos sockets (just like ZeroMQ).

#### Prometheus collectors
If enabled, it exposes metrics to Prometheus. There are two 
independently enabled/disabled parts:
- sender metrics. It exposes metrics calculated from sender update stream 
- Zero MQ metrics. It reads Zero MQ from IRI, calculates and exposes 
TPS, CTPS metrics to Prometheus.

