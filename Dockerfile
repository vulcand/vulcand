FROM golang:onbuild
EXPOSE 8181 8182
# The following line is required to install vulcanctl
RUN make install
