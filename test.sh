#!/bin/bash

go test ./... github.com/thewug/{dml,reqtify} -coverprofile cover.out && 
(
	go tool cover -func=cover.out
	go tool cover -html=cover.out -o cover.html &&
		ln -fs `readlink -f cover.html` /storage/wug/samba/scratch/cover.html
)
