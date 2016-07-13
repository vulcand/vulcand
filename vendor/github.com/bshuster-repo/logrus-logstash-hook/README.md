# Logstash hook for logrus <img src="http://i.imgur.com/hTeVwmJ.png" width="40" height="40" alt=":walrus:" class="emoji" title=":walrus:" /> [![Build Status](https://travis-ci.org/bshuster-repo/logrus-logstash-hook.svg?branch=master)](https://travis-ci.org/bshuster-repo/logrus-logstash-hook)
Use this hook to send the logs to [Logstash](https://www.elastic.co/products/logstash) over both UDP and TCP.

## Usage

```go
package main

import (
        "github.com/Sirupsen/logrus"
        "github.com/bshuster-repo/logrus-logstash-hook"
)

func main() {
        log := logrus.New()
        hook, err := logrus_logstash.NewHook("tcp", "172.17.0.2:9999", "myappName")
        if err != nil {
                log.Fatal(err)
        }
        log.Hooks.Add(hook)
        ctx := log.WithFields(logrus.Fields{
          "method": "main",
        })
        ...
        ctx.Info("Hello World!")
}
```

This is how it will look like:

```ruby
{
    "@timestamp" => "2016-02-29T16:57:23.000Z",
      "@version" => 1,
         "level" => "info",
       "message" => "Hello World!",
        "method" => "main",
          "host" => "172.17.0.1",
          "port" => 45199,
          "type" => "myappName"
}
```
