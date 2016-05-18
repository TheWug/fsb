Deployment directions (apologies, there's no script or anything)

1. build the telegram-fsb go project with GOPATH=/.../telegram-fsb go install fsb
2. Install and configure squid3, and include the provided config patch in /etc/squid3/squid.conf
3. If necessary adjust the cache_dir line in the squid conf, and then run squid3 -z to initialize the disk cache
4. Install and configure apache2, and include the provided config patch in /etc/apache2/httpd.conf or whatever its called
5. 127.0.1.1 is allowed to be accessed through squid, so make sure you're not running anything there you don't want accidentally exposed
6. Install and configure the squid url rewriter and make sure it has a place to do its work
7. link the image converter directory into the apache root
8. ensure permissions are correctly set on all relevant directories (what permissions exactly must be set are config specific but:
   - url rewriter/image converter runs as squid's user, and must write files visible to apache's user
   - make sure each path element is +rx for both of those users
9. restart squid and apache, and launch fsb from the go bin directory (pass --help to see command line invocation)
10. ensure port 80 is accessible to the world wide internet

