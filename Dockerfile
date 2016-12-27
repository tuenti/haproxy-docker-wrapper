FROM haproxy:1.6.2

MAINTAINER Jaime Soriano Pastor <jsoriano@tuenti.com>

COPY haproxy-docker-wrapper /usr/local/bin/haproxy-docker-wrapper

VOLUME /usr/local/etc/haproxy
VOLUME /var/lib/haproxy

CMD ["/usr/local/bin/haproxy-docker-wrapper"]
