Deployment directions

1. use ./make, or manually compile as desired (go build ./daemon/fsb)
2. use ./test, or manually test with go test .../.
3. create a directory suitable for converting video files.
   it's probably a good idea to set up a tmpfs mount or similar for this purpose,
   in the ballpark of 100MB. there is a 9.75MB per file limit, conversions are
   relatively quick, and once they complete and upload to telegram, the stored
   files are removed and are re-sent using their cached file id. You may delete and
   recreate the directory with different settings mid-operation if you need to do so.
4. set up a configuration file. use `./fsb --help` to view the available options.
5. run the bot.
   === As with any bots, be aware of the rules of telegram bots
   === and the way misbehavior could reflect upon your account.

