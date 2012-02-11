# VECTOR DELTA ENGINE DAEMON (VDED)

* Homepage: https://github.com/jbuchbinder/vded
* Twitter: [@jbuchbinder](https://twitter.com/jbuchbinder)

## What it is / What it does

**vded** is a simple REST-type method for tracking deltas of
ever-increasing values (although it handles resets, etc, pretty well).
It is meant to accept submissions of values which increment, then
determine the delta between that value and its predecessor, as well as
figure the rate over time.

It also has the ability to track on/off values, which it refers to as
"switches".

It also de/serializes state to a file, so that history isn't lost when
it restarts.

**vded** is written in [node.js](http://nodejs.org/), for
convenience and (hopefully) speed.

## Building

* Prerequisites (Ubuntu/Debian):
`sudo apt-get install nodejs npm ; sudo npm install gmetric`

## Using

### Switches

`http://localhost:48333/switch?host=HOSTNAME&switch=NAME&value=VALUE&ts=TIMESTAMPINSEC&action=put

* value: ON/on/TRUE/true or OFF/off/FALSE/false
* timestamp: long representation by seconds

`http://localhost:48333/switch?host=HOSTNAME&switch=NAME&action=get

### Vectors

Queries to **vded** are as simple as
`http://localhost:48333/submit?host=HOSTNAME&vector=NAME&value=VALUE&ts=TIMESTAMPINSEC&submit_metric=TRUEORFALSE

It will return a "hash" of values, including:

* `last_diff`: Delta between last data reporting period and this one.
* `per_minute`: Rate per minute since the last piece of data was pushed
   in
* `per_hour`: Rate per hour since the last piece of data was pushed in
* `submit_metric`: (Optional) Whether to enable pushing deltas to
   ganglia through gmetric. Defaults to true. Possible values are:
   TRUE, FALSE, true, false, YES, NO, yes, no, 0, 1

(Please note that host is optional but the rest of the params aren't, so
you might get an error otherwise.)

