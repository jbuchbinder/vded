# VECTOR DELTA ENGINE DAEMON (VDED)

![VDED](https://github.com/jbuchbinder/vded/raw/master/vded_logo.png)

* Homepage: https://github.com/jbuchbinder/vded
* Twitter: [@jbuchbinder](https://twitter.com/jbuchbinder)
* [![Build
  Status](https://secure.travis-ci.org/jbuchbinder/vded.png)](http://travis-ci.org/jbuchbinder/vded)

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

**vded** is written in [Go](http://golang.org/), for
convenience and (hopefully) speed.

## Building

At the moment, VDED should be compiled from golang-tip, since there are
some serious issues with the net/http code in the golang 1.0.2 "stable"
release.

* Prerequisites (Ubuntu/Debian):
```
sudo add-apt-repository ppa:gophers/go && sudo apt-get update && sudo apt-get install golang-tip
```
* Building:
```
go get github.com/jbuchbinder/go-gmetric/gmetric && go build
```

## Using

### CLI Usage

```
Usage of vded:
  -daemon=false: fork off daemon process
  -ghost="localhost": ganglia host(s), comma separated
  -gport=8649: ganglia port
  -gspoof="": ganglia default spoof
  -max=300: maximum number of entries to retain
  -port=48333: port to listen for requests
  -state="/var/lib/vded/state.json": path for save state file
```

### Control

`http://localhost:48333/control?action=serialize`

Serialize all data to disk in JSON format.

`http://localhost:48333/control?action=shutdown`

Serialize data to disk and shut down VDED service.

### Switches

`http://localhost:48333/switch?host=HOSTNAME&switch=NAME&value=VALUE&ts=TIMESTAMPINSEC&action=put`

* value: ON/on/TRUE/true or OFF/off/FALSE/false
* timestamp: long representation by seconds

`http://localhost:48333/switch?host=HOSTNAME&switch=NAME&action=get`

### Vectors

Queries to **vded** are as simple as
`http://localhost:48333/vector?host=HOSTNAME&vector=NAME&value=VALUE&ts=TIMESTAMPINSEC&submit_metric=TRUEORFALSE&units=UNITS&group=GROUP`

That will submit values, and will return OK if successful.

Dumping the value of a vector can be accomplished with

`http://localhost:48333/dumpvector?host=HOSTNAME&vector=NAME`

which will return a "hash" of values, including:

* `last_diff`: Delta between last data reporting period and this one.
* `per_minute`: Rate per minute since the last piece of data was pushed
   in
* `per_hour`: Rate per hour since the last piece of data was pushed in
* `submit_metric`: (Optional) Whether to enable pushing deltas to
   ganglia through gmetric. Defaults to true. Possible values are:
   TRUE, FALSE, true, false, YES, NO, yes, no, 0, 1
* `units`: (Optional) Unit name used when submitting metrics to Ganglia.
   This defaults to use "count" if nothing is specified.
* `group`: (Optional) Name of Ganglia metrics group. Defaults to using
   "vectors" if nothing is specified.

(Please note that host is optional but the rest of the params aren't, so
you might get an error otherwise.)


