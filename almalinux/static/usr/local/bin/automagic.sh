#!/bin/sh

export STAGED=$(bootc status --format json --format-version 1 | yq -pj -oj -I=0 .status.staged)

if [ "$STAGED" != "null"]; then
  echo 'system should reboot';
fi
