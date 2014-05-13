FROM ubuntu:12.04

# Let's install go just like Docker (from source).
RUN apt-get update -q
RUN DEBIAN_FRONTEND=noninteractive apt-get install -qy build-essential curl git
RUN curl -s https://go.googlecode.com/files/go1.2.src.tar.gz | tar -v -C /usr/local -xz
RUN cd /usr/local/go/src && ./make.bash --no-clean 2>&1
RUN apt-get -y -q install bzr

# Set up environment variables.
ENV PATH /usr/local/go/bin:$PATH
ENV GOROOT /usr/local/go
ENV GOPATH /home/goworld
ENV VULCANPATH /home/goworld/src/github.com/mailgun/vulcand

RUN echo "clear cache 4"
RUN go get -v -u github.com/gorilla/mux
RUN go get -v -u github.com/mailgun/vulcan
RUN go get -v -u github.com/mailgun/vulcand
RUN go get -v -u github.com/mailgun/vulcand/vulcanctl
RUN go install github.com/mailgun/vulcand
RUN go install github.com/mailgun/vulcand/vulcanctl
RUN mkdir /opt/vulcan
RUN cp /home/goworld/bin/vulcand /opt/vulcan
RUN cp /home/goworld/bin/vulcanctl /opt/vulcan

