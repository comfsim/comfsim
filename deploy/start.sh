#!/bin/sh  
# Run the server-mode API locally; nginx proxies to it.  
GCSIM_SHARE_KEY="${GCSIM_SHARE_KEY:-}" gcsim-server -host 127.0.0.1 -port 54321 &  
exec nginx -g 'daemon off;'
