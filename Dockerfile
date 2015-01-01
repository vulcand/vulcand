FROM golang:1.4-onbuild
EXPOSE 8181 8182
RUN make install
