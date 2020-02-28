# Prometheus-kopano-exporter

Prometheus is an open source monitoring system for which Kopano offers an
exporter for the kopano-server, kopano-dagent and kopano-spooler.

The prometheus-kopano-exporter accepts data from a unix socket which it creates
on startup and can be specified by with --socket-loc.

The kopano-server or other deamons require the statsclient_url to be set to the
exporter socket and set statsclient_interval to a lower value than the
prometheus scraping interval.

## Grafana

An example Grafana dashboard is available in the Grafana folder and can be
imported using Grafana's webui "create" => "import" => "upload .json file".
