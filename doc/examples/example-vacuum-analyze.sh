#!/bin/sh
# Example script: runs on the schedule given for it in the crontab mounted
# from the Cluster's supercronic-scripts ConfigMap. PGHOST/PGPORT/PGUSER/
# PGPASSWORD are already set by the plugin, so plain `psql` just works.
set -eu

psql -v ON_ERROR_STOP=1 -c "VACUUM (ANALYZE);"
