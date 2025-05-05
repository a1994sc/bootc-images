#! /bin/bash

if (( $UID > 999 )); then
  export PS1='[\[\e[93m\]\u\[\e[97m\]@\h:\[\e[36m\] \W\[\e[0m\]]: '
else
  export PS1='[\[\e[91m\]\u\[\e[97m\]@\h:\[\e[36m\] \W\[\e[0m\]]: '
fi
