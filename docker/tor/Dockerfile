# syntax=docker/dockerfile:1.7

ARG TOR_VERSION=0.4.8.13
ARG TOR_SOURCE=https://dist.torproject.org/tor-${TOR_VERSION}.tar.gz

FROM debian:bookworm AS builder

ARG TOR_VERSION
ARG TOR_SOURCE

RUN apt-get update && \
	DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
		build-essential \
		ca-certificates \
		curl \
		libevent-dev \
		libssl-dev \
		libzstd-dev \
		pkg-config \
		autoconf \
		automake \
		libtool && \
	rm -rf /var/lib/apt/lists/*

WORKDIR /tmp/tor

RUN curl -fsSL ${TOR_SOURCE} -o tor.tar.gz && \
	tar -xf tor.tar.gz --strip-components=1 && \
	./configure --disable-unittests --disable-asciidoc --prefix=/opt/tor && \
	make -j"$(nproc)" && \
	make install

FROM debian:bookworm-slim

ARG TOR_VERSION

RUN apt-get update && \
	DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
		ca-certificates \
		curl && \
	rm -rf /var/lib/apt/lists/*

COPY --from=builder /opt/tor /usr/local

RUN useradd --system --home /var/lib/tor --shell /sbin/nologin tor && \
	mkdir -p /etc/tor /var/lib/tor && \
	mkdir -p /data/state && \
	chown -R tor:tor /etc/tor /var/lib/tor /data

COPY torrc /etc/tor/torrc

USER tor

WORKDIR /data

VOLUME ["/data"]

EXPOSE 9150 9051

ENTRYPOINT ["tor", "-f", "/etc/tor/torrc"]
