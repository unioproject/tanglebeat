## About Tanglebeat
**Tanglebeat** is a lightweight and highly configurable 
software agent. It is aimed to collect Tangle-related metrics to 
[Prometheus TSDB](https://prometheus.io/) do be displayed with such tools
as [Grafana](https://grafana.com). Tanglebeat can be run as one agent or it can be combined in different distributed combinations of unlimited number of individualy configured agents. It allows to ensure high availibility of the measurement as well as objectivity through averaging of several sources. 

It is a successor of [Traveling IOTA](http://traviota.iotalt.com) project, 
scaled up and, hopefully, more practical version of the latter.

## Transfer confirmation metrics

_Tanglebeat_ is performing IOTA value transfers from one address 
to the next one in an the endless loop.  
Tangle beat makes sure the bundle of the transfer is confirmed by _promoting_
it and _reattaching_ (if necessary). 
Immediately after current transfer is confirmed Tanglebeat starts sending 
those iotas to the next address in the sequence. And so on.

Several sequences of addresses is run in parallel. 
Confirmation time and other statistical data is collected in 
the process of sending and, after some averaging, is provided as 
metrics. 

Examples of the metrics are _transfers per hour or TfPH_, _average PoW cost per confirmed transfer_, _average confirmation time_.

Tanglebeat makes available much more data about transfers. It can be used to calculate other metrics and to visualize transfer progress.

## ZeroMQ metrics

Tanglebeat also provides usual metrics derived from data of Zero MQ stream by IRI such as _TPS_, _CTPS_, _Confirmation rate_ and _duration between milestones_

## Highly configurable

Each Tanglebeat agent consist of the following functional 
parts. Every part can be enabled, disabled and configured
independently from each other thus enabling congfiguration of any size and complexity.

#### Sender

Sender for performs transfer bundle generation, promotion and 
(re)attachment until confirmed for each of enabled address sequences. 
Sender's input in config file is config data for each sequences. 
It includes sequence's seed.

Sender if enabled is slow (value) spammer for the Tangle.

Sender generates **updates** with the information 
about the progress, per sequence. Updates contain all the data, 
necessary to calculate various metrics. 
It also allows visualisation of the process 
like [Traveling IOTA](http://traviota.iotalt.com) does.
 
#### Data collector

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

