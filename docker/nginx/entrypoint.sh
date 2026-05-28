#!/bin/sh
set -eu

/usr/local/bin/frozenfortress-cert-bootstrap
exec "$@"