#!/bin/bash

SCRIPTFILE="$(readlink --canonicalize-existing "$0")"
echo "SCRIPTFILE: ${SCRIPTFILE}"
SCRIPTPATH="$(dirname "$SCRIPTFILE")"
echo "SCRIPTPATH: ${SCRIPTPATH}"

ADMIN=${SCRIPTPATH}/../../cmd/admin

echo ""
echo "Initial Plugins"
${ADMIN} plugin add ${SCRIPTPATH}/plugins/*.json

echo ""
echo "Initial Pipelines"
${ADMIN} pipeline add ${SCRIPTPATH}/pipelines/*.json
