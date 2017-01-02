FROM haproxy:1.6.2

MAINTAINER Jaime Soriano Pastor <jsoriano@tuenti.com>

RUN apt-get update \
	&& apt-get install -y iptables libnetfilter-queue1 \
	&& apt-get clean && rm -fr /var/lib/apt/lists/*

COPY haproxy-docker-wrapper /usr/local/bin/haproxy-docker-wrapper

VOLUME /usr/local/etc/haproxy
VOLUME /var/lib/haproxy

CMD ["/usr/local/bin/haproxy-docker-wrapper"]
