FROM golang:1.3.3-onbuild
EXPOSE 8181 8182
# The following line is required to install vctl
RUN make install
