#! /bin/bash

if (( $UID < 1000 )); then
  export PS1="[\[\e[38;5;11m\]\u\[\e[97;11m\]@\h:\[\e[36;11m\] \W\[\e[0;11m\]]: "
else
  export PS1="[\[\e[1;31;11m\]\u\[\e[97;11m\]@\h:\[\e[36;11m\] \W\[\e[0;11m\]]: "
fi
