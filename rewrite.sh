#!/bin/bash

cache_path=/bulk/proxy/fsb2/convert-cache
logfile=/bulk/proxy/fsb2/rewrite.log

umask 177 # files will be created 600
echo "Starting... ($(date))" >> "$logfile"
umask 133 # files will be created 644

while read line; do
	toks=($line)
	url="${toks[0]}"
	echo "<< $url" >> /bulk/proxy/fsb2/rewrite.log

	if [[ "$url" == *"?no-local-redirect=true" ]]
		then
		# provide a way to get a png through the proxy by the image converter itself
		url_nord="$(expr match "$url" '\(.*\)?no-local-redir=true')"
		echo ">> [NO-REDIR] $url_nord" >> "$logfile"
		echo "$url_nord"
	elif [[ "$url" == *".png.jpg" ]]
		then
		url_nojpg="$(expr match "$url" '\(.*\)\.jpg')" # extract real url
		new_filename="$(expr match "$url" '.*/\([0-9a-f]*\..*\)')" # extract target filename
		echo "   [DEBUG   ] url_nojpg = $url_nojpg" >> "$logfile"
		echo "   [DEBUG   ] new_filename = $new_filename" >> "$logfile"
		if [ ! -f "$cache_path"/"$new_filename" ]
		then
			wget -O - "$url_nojpg" | convert - -background white -flatten -quality 90 "/bulk/proxy/fsb2/convert-cache/$new_filename"
		fi
		echo "http://127.0.1.1:81/convert-cache/$new_filename"
		echo ">> [CONVERT ] http://127.0.1.1:81/convert-cache/$new_filename" >> "$logfile"
	else
		echo "$url"
		echo ">> [PASSTHRU] $url" >> "$logfile"
	fi
done;
