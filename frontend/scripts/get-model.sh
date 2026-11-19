#!/bin/sh
# Downloads the 41 MB Vosk small English model into public/models/ (served to the browser, runs on-device).
set -e
cd "$(dirname "$0")/../public/models"
OUT=vosk-model-small-en-us-0.15.tar.gz
[ -f "$OUT" ] && { echo "$OUT already present"; exit 0; }
echo "Downloading Vosk model..."
if curl -fL --retry 3 -o "$OUT" "https://ccoreilly.github.io/vosk-browser/models/vosk-model-small-en-us-0.15.tar.gz"; then
  echo "done: $OUT"; exit 0
fi
echo "Mirror failed, trying alphacephei.com and repacking..."
curl -fL --retry 3 -o m.zip "https://alphacephei.com/vosk/models/vosk-model-small-en-us-0.15.zip"
unzip -q m.zip && tar czf "$OUT" vosk-model-small-en-us-0.15 && rm -rf m.zip vosk-model-small-en-us-0.15
echo "done: $OUT"
