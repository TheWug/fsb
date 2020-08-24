#!/bin/bash

filename="$1"
audio=""
if [[ "$2" == noaudio ]]
then
	audio="-an"
fi

ffmpeg	-i - \
	-f mp4 \
	-vcodec libx264 \
	$audio \
	-pix_fmt yuv420p \
	-preset veryfast \
	-tune zerolatency \
	-crf 25 \
	-threads 3 \
	-movflags faststart \
	-vf "scale=w='400*iw/trunc(max(800,max(iw,ih))/2)':h='400*ih/trunc(max(800,max(iw,ih))/2)':flags=fast_bilinear" \
	"$filename"
