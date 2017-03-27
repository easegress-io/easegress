#!/bin/bash

case $SHELL in
*/zsh)
   # assume Zsh
   FILE=${(%):-%N}
   ;;
*/bash)
   # assume Bash
   FILE=${BASH_SOURCE[0]}
   ;;
*)
   # assume something else
esac

pushd `dirname $FILE` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $FILE`

export GOPATH="${SCRIPTPATH}/_vendor:${SCRIPTPATH}"
