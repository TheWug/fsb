#!/bin/bash

# http://stackoverflow.com/questions/59895/can-a-bash-script-tell-which-directory-it-is-stored-in
cd "$( dirname "${BASH_SOURCE[0]}" )"

if GOPATH=$(pwd) go install fsb;
then
	true;
else
	echo -e "\033[0;31m==========  BUILD FAILED!  ===========\033[0m";
	exit 1;
fi;

echo -e "\033[0;32m==========  BUILD SUCCEED!  ===========\033[0m"
killall fsb 

./bin/fsb "$(echo ~/.fsb.json)" 2> ./err.log &
disown
tail -f knottybot.log err.log
