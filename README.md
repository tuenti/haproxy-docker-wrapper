haproxy-docker-wrapper
======================

This repository contains the code for an haproxy wrapper with these features:

* Embedded syslog server to redirect haproxy logs to standard output
* HTTP entry point to trigger haproxy configuration reloads

It also includes a Dockerfile to extend official haproxy dockers with
this wrapper.

Usage
-----

Start the wrapper passing as arguments the configuration needed for your
environment, the defaults play well with the official haproxy docker image and
the included Dockerfile uses this image as base.

Configuration directory is exposed as a volume, sidecar container should use
this volume. Both containers should also be in the same network namespace, so
it can reach the control entry point without needing to expose it beyond a
local interface.

To trigger a configuration reload, send an HTTP GET request to /reload in the
control entry point (http://127.0.0.1:15000/reload by default).

Haproxy must be configured in *daemon* mode.

Why?
----

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

Credits & Contact
-----------------

`haproxy-docker-wrapper` was created by [Tuenti Technologies S.L.](http://github.com/tuenti)

You can follow Tuenti engineering team on Twitter [@tuentieng](http://twitter.com/tuentieng).

License
-------

`haproxy-docker-wrapper` is available under the Apache License, Version 2.0. See LICENSE file
for more info.

