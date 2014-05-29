[![Build Status](https://drone.io/github.com/mailgun/minheap/status.png)](https://drone.io/github.com/mailgun/minheap/latest)

TtlMap
=======

Redis-like Map with expiry times and maximum capacity

```go

import "github.com/mailgun/ttlmap"

mh := ttlmap.NewMap(20)
mh.Set("key1", "value", 20)
valI, exists := mh.Get("key2")
if exists {
   val := valI.(string)
}
```
