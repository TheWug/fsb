#!/bin/bash

helpmsg () {
    echo "\
Usage: $0 [DIRECTORY] [USERNAME]
    DIRECTORY - the desired location of the cache folder.
                pass this in the config file as media_convert_directory.
                (default: /var/fsb/convert-ramdisk)
    USERNAME - the username that the bot will run as.
               (default: nobody)"
}

if [[ "$1" == "-h" || "$1" == "--help" ]]; then
    helpmsg
    exit
fi

convert_location="${1:-/var/fsb/convert-ramdisk}"
user="${2:-nobody}"
user_id="$(id -u "$user")"
group_id="$(id -g "$user")"

if [[ -z "$user_id" || -z "$group_id" ]]; then
    echo "User [$user] or doesn't exist, or has no group! manually specify a user."
    helpmsg 1>&2
    exit 1
fi

echo "A directory at [$convert_location] will be created, and a mount"
echo "point for user [$user ($user_id)] will be configured."
echo

if [[ $EUID -ne 0 ]]; then
    echo "This script must be run as root."
    exit 1
fi

echo    "Press ENTER to proceed."
read -p "Press CTRL+C to cancel." foo

echo
echo "Writing to /etc/fstab..."
cat <<< "

# FSB tmpfs (deployed from github.com/thewug/fsb: $(pwd))
#<filesystem ID>				<mount>				<type>	<opts>									<dump>	<pass>
tmpfs						$convert_location	tmpfs	rw,noatime,noexec,nodev,nosuid,size=100m,uid=$user_id,gid=$group_id,mode=700	0	0
" | tee -a /etc/fstab
echo "...done. Creating directory..."
mkdir -m 700 -p "$convert_location"
chown "$user:$group" "$convert_location"
echo "...done. Mounting ramdisk..."
mount "$convert_location"
echo "...done. All finished."
