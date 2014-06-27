#!/bin/bash
find . -iregex .*go | xargs sed -i 's:".*Godeps/_workspace/src/:":g'
