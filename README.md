= haproxy-docker-wrapper

This repository contains the code for a wrapper with this features:

* Embedded syslog server to redirect haproxy logs to standard output
* Unix socket to control haproxy reloads

It also includes a Dockerfile to extend official haproxy dockers with
this wrapper.

== Why?

When trying to deploy haproxy in docker, you may face some recurrent problems:

* haproxy only logs to syslog unless it's executed in debug mode, what is not
  recommended for production environments. In docker deployments it is usual to
  log everything to standard output and let logging drivers decide how to
  handle these logs.
* Configuration reload in haproxy needs access to the pid namespace, to the
  pid file and to the configuration file. Providing all this from a sidecar
  container is quite complex. Not using sidecar and including the software
  managing the configuration in the same container makes any update in this
  software to require also a full restart of haproxy.
