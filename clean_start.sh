#!/bin/sh

rm -fr .devconfig/ws1/database
rm -fr .devconfig/ws1/logs
make fullnode
./box start