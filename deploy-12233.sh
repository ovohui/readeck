#!/usr/bin/env bash
# Deploy the customized Readeck as a NEW container on port 12233.
# The original container/image/volume are left untouched.
# Run this on the Docker host (the server that currently runs Readeck on 15351).

set -euo pipefail

# ---- adjustable settings -------------------------------------------------
IMAGE_NAME="readeck-custom:latest"     # new image name; original image untouched
CONTAINER_NAME="readeck-12233"         # new container name
HOST_PORT="12233"                      # host port requested
DATA_VOLUME="readeck-12233-data"       # NEW volume; original data untouched
REPO_URL="https://github.com/ovohui/readeck.git"
SRC_DIR="${SRC_DIR:-/opt/readeck-custom}"

# Custom feature flags
DOWNLOAD_VIDEOS="true"                 # offline video download
VIDEO_QUALITY="720p"                   # best / 1080p / 720p / 480p / worst

# If accessed through a reverse proxy at https://readeck.zh.ovohui.com:12233/,
# uncomment and set the public URL below. Otherwise leave commented for direct
# http://<server-ip>:12233 access.
# PUBLIC_URL="https://readeck.zh.ovohui.com:12233/"
# TRUSTED_PROXIES="172.16.0.0/12,127.0.0.0/8"
# ---------------------------------------------------------------------------

echo "==> Fetching source"
if [ -d "$SRC_DIR/.git" ]; then
  git -C "$SRC_DIR" pull --ff-only
else
  git clone "$REPO_URL" "$SRC_DIR"
fi
cd "$SRC_DIR"

echo "==> Building image $IMAGE_NAME (first build downloads Go/Node deps, may take several minutes)"
docker build -t "$IMAGE_NAME" -f tools/Containerfile .

echo "==> Removing any previous container named $CONTAINER_NAME (only this new one)"
docker rm -f "$CONTAINER_NAME" 2>/dev/null || true

echo "==> Starting $CONTAINER_NAME on port $HOST_PORT"
RUN_ARGS=(
  -d
  --name "$CONTAINER_NAME"
  --restart unless-stopped
  -p "${HOST_PORT}:8000"
  -v "${DATA_VOLUME}:/readeck"
  -e "READECK_SERVER_HOST=0.0.0.0"
  -e "READECK_DOWNLOAD_VIDEOS=${DOWNLOAD_VIDEOS}"
  -e "READECK_VIDEO_QUALITY=${VIDEO_QUALITY}"
)

if [ -n "${PUBLIC_URL:-}" ]; then
  RUN_ARGS+=("-e" "READECK_SERVER_BASE_URL=${PUBLIC_URL}")
  RUN_ARGS+=("-e" "READECK_SERVER_TRUSTED_PROXIES=${TRUSTED_PROXIES:-172.16.0.0/12}")
fi

docker run "${RUN_ARGS[@]}" "$IMAGE_NAME"

echo "==> Status"
docker ps --filter "name=${CONTAINER_NAME}" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
echo ""
echo "Open http://<server-ip>:${HOST_PORT}/ and finish the onboarding (create admin account)."
echo "Then go to Settings > Site Cookies and add the xiaohongshu.com cookie."
