FROM golang:1.8-onbuild
EXPOSE 8181 8182
RUN make install
ENTRYPOINT ["/go/bin/vulcand"]
