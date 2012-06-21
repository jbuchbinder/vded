#!/bin/bash

wget -q -O /dev/stdout 'http://localhost:48333/vector?host=localhost&vector=test&value=10&ts=10'
echo " "
wget -q -O /dev/stdout 'http://localhost:48333/vector?host=localhost&vector=test&value=300&ts=25'
echo " "
wget -q -O /dev/stdout 'http://localhost:48333/dumpvector?host=localhost&vector=test'
echo " "

