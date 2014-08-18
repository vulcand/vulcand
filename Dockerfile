FROM ubuntu:12.04

# Let's install go just like Docker (from source).
RUN apt-get update -q
RUN DEBIAN_FRONTEND=noninteractive apt-get install -qy build-essential curl git
RUN curl -s https://storage.googleapis.com/golang/go1.3.1.src.tar.gz | tar -v -C /usr/local -xz
RUN cd /usr/local/go/src && ./make.bash --no-clean 2>&1

# Set up environment variables.
ENV PATH /usr/local/go/bin:$PATH
ENV GOROOT /usr/local/go
ENV GOPATH /home/goworld
ENV VULCANPATH /home/goworld/src/github.com/mailgun/vulcand

RUN echo "Rebuild image on 2014 August, 16th 00:10"
ADD . $VULCANPATH
RUN cd $VULCANPATH && make install
RUN mkdir /opt/vulcan
RUN cp /home/goworld/bin/vulcand /opt/vulcan
RUN cp /home/goworld/bin/vulcanctl /opt/vulcan

RUN echo "Cleanup"
RUN rm -rf /usr/local/go /home/goworld
