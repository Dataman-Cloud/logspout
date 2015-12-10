#!/bin/sh
crond -f & 
/bin/logspout tcp://123.59.58.58:5000
