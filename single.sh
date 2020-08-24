#!/bin/bash

filename="$1"
audio=""
if [[ "$2" == noaudio ]]
then
	audio="-an"
fi

ffmpeg	-i - \
	-y \
	-f mp4 \
	-vcodec libx264 \
	$audio \
	-pix_fmt yuv420p \
	-preset veryfast \
	-tune zerolatency \
	-crf 25 \
	-fs 9.75M \
	-protocol_whitelist -all,file \
	-threads 3 \
	-movflags faststart \
	-vf "scale=w='2*trunc(800*iw/(2*max(800,max(iw,ih))))':h='2*trunc(800*ih/(2*max(800,max(iw,ih))))':flags=fast_bilinear" \
	"$filename"
