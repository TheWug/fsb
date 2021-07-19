#!/bin/bash

# http://stackoverflow.com/questions/59895/can-a-bash-script-tell-which-directory-it-is-stored-in
cd "$( dirname "${BASH_SOURCE[0]}" )"

if GOROOT= GOPATH=$(pwd) /usr/bin/go install fsb;
then
	true;
else
	echo -e "\033[0;31m==========  BUILD FAILED!  ===========\033[0m";
	exit 1;
fi;

echo -e "\033[0;32m==========  BUILD SUCCEED!  ===========\033[0m"
