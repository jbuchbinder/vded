# VECTOR DELTA ENGINE DAEMON (VDED)

* Homepage: https://github.com/jbuchbinder/vded
* Twitter: [@jbuchbinder](https://twitter.com/jbuchbinder)

## What it is / What it does

**vded** is a simple REST-type method for tracking deltas of
ever-increasing values (although it handles resets, etc, pretty well).
It is meant to accept submissions of values which increment, then
determine the delta between that value and its predecessor, as well as
figure the rate over time.

At some point, it'll also de/serialize state to a file, so that history
isn't lost when it restarts, etc.

**vded** is written in [Vala](http://live.gnome.org/Vala), for
convenience and (hopefully) speed.

## Building

* Prerequisites (Ubuntu/Debian):
`sudo apt-get install valac-0.14 libgee-dev libsoup2.4-dev`
* Building:
`make`

## Using

Queries to **vded** are as simple as
`http://localhost:48333/submit?host=HOSTNAME&vector=NAME&value=VALUE&ts=TIMESTAMPINSEC`

It will return a "hash" of values, including:

* `last_diff`: Delta between last data reporting period and this one.
* `per_minute`: Rate per minute since the last piece of data was pushed
   in

(Please note that host is optional but the rest of the params aren't, so
you might get an error otherwise.)

