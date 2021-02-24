# Prometheus-kopano-exporter

Prometheus is an open source monitoring system for which Kopano offers an exporter for `kopano-server`, `kopano-dagent` and `kopano-spooler`.

The `prometheus-kopano-exporter` accepts data from a unix socket, which it creates on startup and can be specified by passing the `--listen-socket` argument (or `prometheus-kopano-exporter.cfg`).

`kopano-server` or other deamons need to be configured to send statistics data to `prometheus-kopano-exporter` by setting `statsclient_url` to the socket created by `prometheus-kopano-exporter` and by setting `statsclient_interval` to a lower value than the prometheus scraping interval.

## Grafana

An example Grafana dashboard is available in the Grafana folder and can be imported using Grafana's webui through "create" => "import" => "upload .json file".
