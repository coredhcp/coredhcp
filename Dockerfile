FROM ubuntu:22.04

LABEL BUILD="docker build -t coredhcp/coredhcp -f Dockerfile ."
LABEL RUN="docker run --rm -it coredhcp/coredhcp"

# Install dependencies
RUN apt-get update &&                          \
    apt-get install -y --no-install-recommends \
        sudo \
	iproute2 \
        # to fetch the Go toolchain
        ca-certificates \
        wget \
        # for go get
        git \
	# for CGo support
	build-essential \
        && \
    rm -rf /var/lib/apt/lists/*

# install Go
WORKDIR /tmp
RUN set -exu; \
    wget https://golang.org/dl/go1.23.4.linux-amd64.tar.gz ;\
    tar -C / -xvzf go1.23.4.linux-amd64.tar.gz
ENV PATH="$PATH:/go/bin:/build/bin"
ENV GOPATH=/go:/build

ENV PROJDIR=/build/src/github.com/coredhcp/coredhcp
RUN mkdir -p $PROJDIR
COPY . $PROJDIR

# build coredhcp
RUN set -exu ;\
    cd $PROJDIR/cmds/coredhcp ;\
    go get -v ./... ;\
    CGO_ENABLED=1 go build ;\
    cp coredhcp /bin

EXPOSE 67/udp
EXPOSE 547/udp

CMD coredhcp --conf /etc/coredhcp/config.yaml
