#!/usr/bin/env bash

dep ensure
dep prune

# dep prune does poor job so we have to cleanup some unused source files.
rm -f vendor/github.com/coreos/etcd/main.go
find vendor/github.com/coreos/etcd/auth -type f -maxdepth 1 -delete
find vendor/github.com/coreos/etcd/etcdserver/api/v3rpc/ -type f -maxdepth 1 -delete
find vendor/github.com/coreos/etcd/etcdserver/api/ -type f -maxdepth 1 -delete
find vendor/github.com/coreos/etcd/etcdserver -type f -maxdepth 1 -delete
find vendor/github.com/coreos/etcd/mvcc -type f -maxdepth 1 -delete
find vendor/golang.org/x/crypto/salsa20/ -type f -maxdepth 1 -delete
find vendor/golang.org/x/crypto/ssh/ -type f -maxdepth 1 -delete
rm -rf vendor/golang.org/x/sys/windows
find vendor/gopkg.in/mgo.v2 -type f -maxdepth 1 -delete
find vendor/ -name *_test.go -delete
